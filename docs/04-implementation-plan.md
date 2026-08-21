# Implementation Plan

## Stack

- Go 1.26+
- PostgreSQL 15+
- pgx/v5 with hand-written SQL (no ORM)
- goose for forward-only SQL migrations (run via `go tool goose`)
- godotenv for `.env` loading

## Project Structure

```txt
mlbb-analyzer-service/
  cmd/
    seed/            seeder binary: data/static -> Postgres, idempotent
    api/             REST API binary consumed by the frontend
  internal/
    api/             HTTP layer: routes, handlers, middleware, Swagger
    analyzer/        AI scoring: prompts, provider chain, cache
    store/           Postgres access: sync + read queries
    staticdata/      dataset loading and file-level validation
  data/
    static/          hand-authored dataset, source of truth in git
  migrations/        numbered .sql files with goose markers
  Makefile
```

## Data Flow

1. `make migrate-up` applies the schema.
2. `make seed` syncs `data/static/` into Postgres: upsert present rows, delete
   rows removed from the files, all in one transaction. Database constraints
   enforce the dataset contract; the loader enforces the file-level rules the
   database cannot see.
3. `make api` serves the REST contract from Postgres. AI analyze endpoints
   call the configured provider chain and cache successful responses in-process.

## Verification

```bash
make test
```

Store and API tests run against local Postgres inside rolled-back transactions.
