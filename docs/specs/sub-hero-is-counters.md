# sub-hero-is-counters

## Goal

Model Moonton's `sub_hero` / `sub_hero_last` as what they are — a directed
counter matchup — instead of as synergy, correcting the staging view, the mart,
and the Go types that read them.

The relation is antisymmetric, verified over a full 133-hero pull: of 665
`sub_hero` pairs, zero are mutual and 347 (52%) appear in the partner's
`sub_hero_last`. Synergy cannot produce that. mobilelegends.com renders this
same field under a COUNTER HERO column, fetched with the field list this
collector already used.

`main_heroid` is the hero being countered and a `sub_hero` entry is the hero
countering it, which lines up exactly with the authored `public.counters`
convention of `target_hero_id` / `counter_hero_id`.

## Non-goals

- The authored dataset. `public.counters`, `public.synergies`, their proof
  tables, `data/static`, the analyzer prompt and every `/api` route stay
  untouched: hand-authored synergies remain synergies.
- `raw.hero_rank_snapshots`. The payload is what upstream sent; only its
  interpretation was wrong. No re-collection, no backfill, no schema change.
- `marts.hero_current` and its patch columns.
- New API routes. The counter mart has no HTTP surface in this work.
- Feeding measured counters into AI scoring. That is
  `empirical-evidence-in-scoring`, which this unblocks but does not deliver.

## Decisions

- **One directed relation.** `staging.hero_counter_daily` carries one row per
  ordered pair, always oriented so `counter_heroid` beats `target_heroid` by
  `win_rate_delta`. `sub_hero` rows keep the upstream orientation;
  `sub_hero_last` rows are flipped and their delta negated. A `source` column
  records which list a row came from, so the two halves stay separable.
- **0008 and 0009 are rewritten in place**, not superseded by a forward
  migration. Nothing has been deployed, so these migrations were never
  published: the misnaming is a bug in unreleased code. Rolling back to 0007
  keeps every hero snapshot and costs only the patch rows, which currently hold
  no highlights and are better re-collected.
- **The mart is rebuilt, not dropped.** `marts.hero_counter_current` has no
  reader today but a known next one: counters had no pairwise measurement, which
  is what blocked the counter-evidence decision in the scoring spec.
- Historical snapshots carry only `sub_hero`, because `sub_hero_last` was not
  requested until the collector switched to the server's default projection. The
  `source` column makes that visible rather than silent; no backfill is possible.

## Acceptance criteria

- AC-1: No database object, Go identifier, or migration outside the authored
  dataset describes `sub_hero` data as synergy. `grep -rn "hero_synergy_daily\|hero_synergy_current\|HeroSynergyStat\|win_rate_lift\|partner_rank" internal/ migrations/` returns nothing.
- AC-2: `staging.hero_counter_daily` exposes `target_heroid`, `counter_heroid`,
  `win_rate_delta`, `source`, `rank_tier`, `window_days`, `snapshot_date`.
- AC-3: Every row satisfies `win_rate_delta > 0` — the orientation invariant.
  A `sub_hero_last` row appears with its two hero ids swapped relative to
  upstream and its delta sign flipped.
- AC-4: `marts.hero_counter_current` returns the newest snapshot per
  (target, counter, tier, window), with both heroes' uid and name joined and
  NULL for heroes the authored dataset does not carry.
- AC-5: `domain.HeroCounterStat` and `postgres.HeroCounterStats` replace the
  synergy pair, keyed on target hero id, and are covered by tests that assert
  the orientation rather than only the round trip.
- AC-6: The authored path is untouched: `git diff` reports no change under
  `data/static/`, `internal/dataset/`, `internal/analyzer/`, `internal/api/`,
  or migrations 0003–0006.
- AC-7: `go build ./...`, `go vet ./...`, `gofmt -l ./cmd ./internal` and
  `go test ./...` are clean.

## Verification

```
make migrate-down && make migrate-down && make migrate-down && make migrate-down && make migrate-down
make migrate-up
make collect-patches
grep -rn "hero_synergy_daily\|hero_synergy_current\|HeroSynergyStat\|win_rate_lift\|partner_rank" internal/ migrations/
go build ./... && go vet ./... && gofmt -l ./cmd ./internal && go test ./... 2>&1 | grep -v 'no test files'
psql "$DB_URL" -c "\d+ staging.hero_counter_daily"
psql "$DB_URL" -tAc "SELECT count(*) FROM staging.hero_counter_daily WHERE win_rate_delta <= 0"
psql "$DB_URL" -tAc "SELECT source, count(*) FROM staging.hero_counter_daily GROUP BY 1"
git diff --stat -- data/static internal/dataset internal/analyzer internal/api migrations/0003_counters.sql migrations/0004_counter_proofs.sql migrations/0005_synergies.sql migrations/0006_synergy_proofs.sql
```
