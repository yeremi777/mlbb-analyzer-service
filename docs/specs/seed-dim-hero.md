# Spec: seed dim_hero from heroes.json

## Goal

A Go binary `cmd/seed` that idempotently syncs `app/data/static/heroes.json` (132 heroes) into `marts.dim_hero` in local Postgres. Git is the source of truth: rows are upserted, and rows removed from the JSON are deleted from the table.

## Non-goals

- Counter, proof, and synergy tables (later migrations/slices).
- The collector, raw zone, dbt, and the Go API.
- Adding Hirara to heroes.json (authoring work, separate change).

## Decisions

- One database, layered by schemas: `raw`, `staging`, `marts`. `dim_hero` lives in `marts`.
- pgx/v5 (>= v5.9.2), hand-written SQL, no ORM. `SendBatch` for the upsert.
- Sync inside one transaction: upsert all present rows, then delete rows not present.
- Refuse to run when the source parses to zero heroes (a broken file must not wipe the table).
- Refuse duplicate `uid` in the source (would raise a cardinality violation mid-batch).
- Config from env: `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USERNAME`, `DB_PASSWORD` (matches `.env.example`), loaded via godotenv when a `.env` file exists.
- `-data` flag overrides the dataset directory (default `app/data/static`) so sync-delete can be tested against a trimmed copy without touching the real files.
- `updated_at` bumps only when a row actually changes (`IS DISTINCT FROM` guard) so idempotency is observable.

## Acceptance criteria

1. `go run ./cmd/seed` exits 0 and reports 132 heroes synced.
2. `SELECT count(*) FROM marts.dim_hero` = 132.
3. Second run: exit 0, count still 132, `max(updated_at)` unchanged.
4. Run against a copy with one hero removed: that row is gone, count 131.
5. Run against an empty/invalid heroes.json: non-zero exit, table untouched.

## Verification

```bash
go run ./cmd/seed
PGPASSWORD=root psql -h 127.0.0.1 -U postgres -d mlbb_analyzer -tAc "SELECT count(*) FROM marts.dim_hero"
```
