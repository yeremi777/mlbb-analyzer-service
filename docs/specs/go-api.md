# Spec: Go REST API replacing FastAPI

## Goal

`cmd/api` serves the existing REST contract from Postgres (`marts.*`), byte-compatible with the Python service so the frontend needs no change. Read endpoints first, AI analyze endpoints second.

## Non-goals

- Redis rate limiting (config-off today; ported later).
- The relay AI provider (openrouter + opencode_zen only).
- OpenAPI/Swagger generation.
- Collector, dbt, deployment.

## Decisions

- stdlib net/http (Go 1.22 mux), no framework. pgxpool for the connection.
- Error shape: `{"error":{"code":"...","message":"..."}}`, same codes as Python.
- `mlid` serializes as a JSON string (contract) from an integer column.
- Counters/synergies list order: deterministic by counter/synergy hero id (authoring file order is not stored; frontend re-sorts by score anyway).
- Same env vars as Python `.env` plus `API_PORT` (default 8080) so both servers can run side by side during cutover.
- CORS: allow FRONTEND_ORIGIN + localhost:3000, credentials on.

## Acceptance criteria

1. `GET /health` → `{"status":"ok"}`.
2. `GET /api/heroes?search=&role=&lane=&page=&size=` matches Python semantics: casefold search on name, exact casefold role/lane match, 1-based pages, size 1..100, `pages=ceil(total/size)`.
3. `GET /api/heroes/{id}` → hero JSON; unknown id → 404 `hero_not_found`.
4. `GET /api/heroes/{id}/counters` → list of `{targetHeroId, counterHero, reasons, counterTypes, proof[]}`; unknown hero → 404 `hero_not_found`; hero without data → 404 `counter_data_not_found`.
5. `GET /api/heroes/{id}/synergies` → same shape with anchor/synergyHero; 404 codes `hero_not_found` / `synergy_data_not_found`.
6. Parity diff: for a sample of heroes, Go responses equal Python responses up to list ordering.
7. AI: `POST /api/counters/analyze-score`, `/api/counters/analyze-detail`, `/api/synergies/analyze-score`, `/api/synergies/analyze-detail` reproduce the Python request/response contract, provider chain openrouter→opencode_zen, in-process TTL cache.

## Verification

```bash
go test ./...
go run ./cmd/api &  # then curl each endpoint; diff against uv run python -m app.server
```

## Post-cutover decision: API documentation tooling

Documentation is generated from swaggo/swag annotations on the handlers
(`make docs` regenerates `internal/docs/`), rendered by Swagger UI at `/docs`.
The spec can never drift from the code because it is derived from it; the
hand-maintained `openapi.json` inherited from the Python service is retired.
