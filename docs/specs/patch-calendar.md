# patch-calendar

## Goal

Give every hero statistic a patch context: a `dim_patch` calendar refreshed daily
from Liquipedia, joined by date range into `marts.hero_current` so a caller can
tell which patch a number reflects and whether its trailing window is
contaminated by pre-patch days.

## Non-goals

- Writing `patch_version` into `raw.hero_rank_snapshots`. Raw records what
  Moonton said; patch is derived from a different source and joins in staging,
  so a corrected patch date fixes all history instead of requiring a rewrite.
- Items, emblems, builds, or any hero-coaching source.
- `marts.patch_impact` (per-hero win-rate movement across a patch boundary).
  Deferred until the calendar is proven in use.
- HTTP endpoints. Nothing in the API surface changes.
- The hero collector's fetch loop, combos, or schedule.
- Rank-tier-specific patch analysis.
- Resolving why the 1-day window did not move between the 2026-08-21 and
  2026-08-22 collections. Tracked separately; it does not block this work.

## Acceptance criteria

- AC-1: `raw.patch_snapshots` holds one row per released patch, keyed on
  `(release_date, version)`, and `staging.patch_calendar` resolves it to one
  row per release date, newest fetch winning. `public` stays authored data
  only. The release date is the natural key, not the version: Liquipedia lists
  38 of 75 versions under several release dates each (1.9.42 ships five times)
  because patches before 2025 reused a version number for successive updates,
  while every release date is distinct. A re-run must not accumulate copies.
- AC-2: The patch collector reads Liquipedia's MediaWiki API and upserts the
  calendar; running it twice on an unchanged page writes nothing and reports
  zero rows changed, while a changed version for a known date lands as a new
  row that `staging.patch_calendar` resolves to, and edited highlights for a
  known patch are taken in place rather than added as a second row. See
  `docs/adr/0002-patch-calendar-upsert.md`.
- AC-3: A Liquipedia failure (transport error, non-200, unparseable body) exits
  non-zero and leaves the existing calendar intact.
- AC-4: The collector sends a descriptive `User-Agent` carrying contact details
  and issues at most one `action=parse` request per run, per Liquipedia's terms.
- AC-5: `marts.hero_current` carries `patch_as_of`, `days_since_patch`, and
  `window_crosses_patch`.
- AC-6: `patch_as_of` is the latest patch whose `released_on` is on or before
  `snapshot_date`; `window_crosses_patch` is true exactly when
  `snapshot_date - window_days < released_on` of that patch.
- AC-7: A snapshot dated before the earliest known patch yields NULL for all
  three patch columns rather than attributing it to a wrong patch.
- AC-8: Existing `marts.hero_current` row count and non-patch column values are
  unchanged by the join: 3990 rows for the current data, same win rates.
- AC-9: One cron entry at 07:00 runs `collector all`, which fetches the patch
  calendar before hero statistics. A patch fetch that fails does not stop the
  statistics run: `patch_as_of` is resolved by a read-time join in
  `marts.hero_current`, so the next successful patch fetch repairs the
  attribution of every snapshot taken in the meantime.

## Verification

go test ./... 2>&1 | grep -v 'no test files'
go build ./... && go vet ./... && gofmt -l ./cmd ./internal
make migrate-up && psql "$DB_URL" -c "\d raw.patches"
go run ./cmd/collector patches && go run ./cmd/collector patches
psql "$DB_URL" -tAc "SELECT count(*), count(DISTINCT released_on) FROM staging.patch_calendar"
LIQUIPEDIA_BASE_URL=http://127.0.0.1:1 go run ./cmd/collector patches; echo "exit=$?"
psql "$DB_URL" -tAc "SELECT count(*) FROM marts.hero_current WHERE patch_as_of IS NULL"
psql "$DB_URL" -tAc "SELECT count(*) FROM marts.hero_current WHERE window_crosses_patch AND snapshot_date - window_days >= (SELECT released_on FROM staging.patch_calendar WHERE version = patch_as_of)"
psql "$DB_URL" -tAc "SELECT count(*) FROM marts.hero_current"
