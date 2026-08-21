# MLBB Analyzer Service

Go backend service for an MLBB counter-pick analyzer.

The service seeds a hand-authored hero, counter, and synergy dataset from `data/static/` into Postgres, validates it through database constraints, and exposes a REST API for the frontend hero selector, counter reveal, and AI analysis flow.

## Features

- Hero list API with search, role, lane, and pagination filters
- Hero detail API
- Hero counter matchup API
- Hero synergy matchup API
- AI counter scoring and detail endpoints (`POST /api/counters/analyze-score`, `POST /api/counters/analyze-detail`)
- AI synergy scoring and detail endpoints (`POST /api/synergies/analyze-score`, `POST /api/synergies/analyze-detail`)
- Idempotent dataset seeding (`cmd/seed`) with sync-to-git semantics
- Versioned SQL migrations via goose
- Swagger UI at `/docs`

## Tech Stack

- Go 1.26+
- PostgreSQL 15+
- pgx/v5 (hand-written SQL, no ORM)
- goose (migrations, run via `go tool goose`)

## Getting Started

Create `.env` from the example and point it at your local Postgres:

```bash
cp .env.example .env
```

Apply migrations and seed the dataset:

```bash
make migrate-up
make seed
```

Run the API:

```bash
make api
```

The API runs at `http://127.0.0.1:8080` (override with `API_HOST` / `API_PORT`).

Swagger docs: `http://127.0.0.1:8080/docs`

## Project Layout

```
cmd/seed         seeder binary: data/static -> Postgres, idempotent
cmd/api          REST API binary: the gateway the frontend consumes
internal/api     HTTP gateway: routes, handlers, middleware, Swagger annotations
internal/analyzer AI scoring: prompts, provider chain, cache
internal/store   Postgres access: sync + read queries
internal/staticdata  dataset types, loading, file-level validation
internal/ratelimit   Redis-backed analyze rate limiting
internal/config  environment configuration helpers
internal/docs    generated OpenAPI spec (make docs)
data/static      hand-authored hero/counter/synergy dataset (source of truth in git)
migrations       forward-only SQL migrations (goose)
```

## Testing

```bash
make test
```

Store and API tests run against the local Postgres in rolled-back transactions.

## Notice

This is a fan-made project. Not affiliated with or endorsed by Moonton.
