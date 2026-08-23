# Spec: collector and raw zone

## Goal

`cmd/collector` fetches daily hero statistics (win rate, appearance share, ban rate) for all rank tiers and windows from Moonton's first-party endpoint and appends them to `raw.hero_rank_snapshots`. Scheduled via cron on the VPS. Every run is idempotent-safe by design: re-runs append, nothing is lost, downstream dedupes.

## Non-goals

- Transformations, staging models, marts, dbt (next arc).
- API changes (no new endpoints; swaggo adoption waits for the first new endpoint).
- Patch stamping at ingest — patch joins from a future `dim_patch` by date range in staging, never written into raw.
- Hosted Postgres — the collector talks to the VPS's own Postgres.
- Parsing `sub_hero` — it is captured inside `payload` and modelled later. It holds counter matchups, not synergies; see `sub-hero-is-counters`.

## Upstream facts (verified 2026-08-21, docs/research/upstream-data-sources.md)

- `POST https://api.gms.moontontech.com/api/gms/source/2669606/{endpointId}` — no auth, no signature.
- Endpoint id per window: 1d=2756567, 3d=2756568, 7d=2756569, 15d=2756565, 30d=2756570.
- Rank filter via `bigrank`: all=101, epic=5, legend=6, mythic=7, honor=8, glory=9. `glory` is the site's "Mythical Glory+" and already includes Mythical Immortal; no higher tier is published.
- One request with `pageSize: 200` returns every hero; no pagination.
- Rates are floats in 0–1. `main_hero_appearance_rate` is share-of-picks (sums to 1.0 across heroes) — store as `appearance_share`, never call it pick rate. `main_hero_ban_rate` is a conventional rate.
- `main_heroid` equals the dataset's `mlid`. Upstream currently has 133 heroes (Hirara) while `data/static` has 132 — raw accepts unknown heroes by design.
- LANDMINE: errors come back as HTTP 200 with `code: 0`, `records: null`, `total: 0`. Success must be asserted on record count, never on HTTP status.

## Decisions

- Migration `0007_raw_hero_rank_snapshots.sql`:

```sql
CREATE TABLE raw.hero_rank_snapshots (
    id               BIGSERIAL PRIMARY KEY,
    main_heroid      INTEGER     NOT NULL,
    rank_tier        TEXT        NOT NULL,  -- all|epic|legend|mythic|honor|glory
    window_days      SMALLINT    NOT NULL,  -- 1|3|7|15|30
    snapshot_date    DATE        NOT NULL,  -- local date of the fetch
    win_rate         NUMERIC(9,6) NOT NULL,
    appearance_share NUMERIC(9,6) NOT NULL,
    ban_rate         NUMERIC(9,6) NOT NULL,
    payload          JSONB       NOT NULL,  -- full per-hero upstream record, unmodified (includes sub_hero)
    fetched_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX hero_rank_snapshots_lookup_idx
    ON raw.hero_rank_snapshots (main_heroid, rank_tier, window_days, snapshot_date);
```

- **The request names no fields.** `"fields": []` returns the server's own
  default projection, which is wider than any list we would write: it carries
  `sub_hero_last` (the five heroes this one counters), per-duration win rates on every
  pairing, and `camp_type`. A narrowed list cannot be repaired later because the
  omitted fields never arrive, and raw exists to record what the source said.
  Payload grows from ~1.4 KB to ~5.7 KB per hero, which crosses the 2 KB TOAST
  threshold and is compressed on the way in, so stored size rises by roughly a
  third rather than fourfold.

- **Append-only, no unique constraint.** Every fetch inserts. A same-day re-run
  produces additional rows; staging dedupes by latest `fetched_at` per
  (heroid, tier, window, date). Upstream restatements stay visible instead of
  being silently discarded. Never UPDATE, never DELETE.
- **No FK to heroes.** Raw accepts whatever upstream sends (Hirara case). A
  future staging model quarantines unknown ids and a test warns.
- Collector loops all 30 tier x window combos: sequential, 2s pause between
  requests, one retry with backoff on transport error or 5xx. ~1 minute total.
- Success guard per combo: response `code == 0` AND `total > 100` AND
  `len(records) == total`. The first two are authoritative (no retry); a short
  read is transient (retried once). Anything else logs the combo as failed and the run
  exits non-zero after finishing remaining combos (partial data is kept —
  append-only makes partial runs safe).
- Politeness: `User-Agent: mlbb-analyzer-collector/1.0 (+github.com/yeremi777/mlbb-analyzer-service; kuroganehunter99@gmail.com)`.
- Structured logging per run: combos fetched, rows inserted, failures.
- `-date YYYY-MM-DD` flag overrides `snapshot_date` (manual catch-up labelling;
  upstream cannot backfill, so this only annotates late fetches honestly).
- Reuses `internal/postgres` (add `InsertSnapshots(ctx, tx, rows)` batching with
  `SendBatch`), new `internal/upstream/moonton` package for the Moonton client.
- cron: daily 07:00 `CRON_TZ=Asia/Jakarta`, wrapped in `flock` so a slow run
  never overlaps the next. Cron does not catch up missed runs.
  `make collect` runs every feed manually; `make collect-stats` runs this one alone. Entry checked into `deploy/cron/` as a
  template; `make collector-install` copies + loads it.

## Acceptance criteria

1. `make migrate-up` creates `raw.hero_rank_snapshots`.
2. `make collect` exits 0 and inserts ~30 x 133 rows for today; log reports combos and row counts.
3. Re-running `make collect` the same day appends again (no constraint error, no dedupe at raw level); count roughly doubles.
4. Kill the network mid-run (or point at a bad URL): run exits non-zero, rows from completed combos remain.
5. A response with `records: null` is detected as failure for that combo (unit test with a stub server), never written as zero rows + success.
6. `SELECT sum(appearance_share) ... WHERE rank_tier='all' AND window_days=1 AND snapshot_date=current_date` is ~1.0 for the latest fetch batch.
7. The cron entry installs and fires; `crontab -l` shows it and `/var/log/mlbb-analyzer/collector.log` records the run.

## Verification

```bash
make migrate-up && make collect
psql ... -c "SELECT rank_tier, window_days, count(*) FROM raw.hero_rank_snapshots WHERE snapshot_date = current_date GROUP BY 1,2 ORDER BY 1,2"
go test ./internal/upstream/moonton/ ./internal/postgres/
```
