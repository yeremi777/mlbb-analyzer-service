# Spec: complete static-data migration into Postgres

## Goal

All authored static data lives in `marts`: `dim_hero.mlid` becomes INTEGER, and the 660 counter matchups (+660 proofs) and 660 synergy matchups (+660 proofs) are seeded idempotently by `cmd/seed`, same sync semantics as heroes.

## Non-goals

- The collector, raw zone, fact tables, dbt.
- The Go API (separate spec).
- Editing heroes.json (mlid stays string in JSON while Python service lives).

## Decisions

- One migration per table: 0003 mlid→INTEGER, 0004 dim_counter, 0005 dim_counter_proof, 0006 dim_synergy, 0007 dim_synergy_proof.
- Constraints in DB: FKs to dim_hero(uid) ON DELETE CASCADE from matchups, proof FK to its matchup; CHECK no self-pair; CHECK non-empty reasons/types arrays; CHECK category/priority/impact allowed values.
- Matchup PK: (target_hero_id, counter_hero_id) / (anchor_hero_id, synergy_hero_id). Proof PK: id (globally unique today, enforced).
- Seeder syncs all five tables in ONE transaction: heroes, then counters+proofs, then synergies+proofs. Delete-not-present per table.
- File-level rules stay in Go: index files list the split files; filename must match targetHeroId/anchorHeroId; refuse empty source.

## Acceptance criteria

1. `go run ./cmd/seed` exits 0; counts: dim_hero 132, dim_counter 660, dim_counter_proof 660, dim_synergy 660, dim_synergy_proof 660.
2. `mlid` column type is integer; `SELECT max(mlid)` = 132.
3. Re-run: all counts unchanged, no `updated_at` churn on identical data.
4. Removing one counter matchup from a copy and seeding from it deletes the matchup and its proof (cascade), count 659.
5. A proof with an invalid category in a copy fails the run; no table changed.

## Verification

```bash
go test ./...
go run ./cmd/seed
PGPASSWORD=root psql -h 127.0.0.1 -U postgres -d mlbb_analyzer -tAc "SELECT (SELECT count(*) FROM marts.dim_hero), (SELECT count(*) FROM marts.dim_counter), (SELECT count(*) FROM marts.dim_counter_proof), (SELECT count(*) FROM marts.dim_synergy), (SELECT count(*) FROM marts.dim_synergy_proof)"
```
