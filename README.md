# MLBB Analyzer Service

FastAPI backend service for an MLBB counter-pick analyzer.

The service loads a local hero and counter dataset, validates it on startup, and exposes API endpoints for the frontend hero selector and counter recommendation flow.

## Features

- Hero list API with search, role, lane, and pagination filters
- Hero detail API
- Hero counter matchup API
- Dataset validation at startup
- Swagger/OpenAPI documentation
- Fast AI scoring endpoint for all counters (`POST /api/counters/analyze-score`)
- On-demand AI detail endpoint per counter card (`POST /api/counters/analyze-detail`)

## Tech Stack

- Python 3.12+
- FastAPI
- Pydantic v2
- uv
- pytest

## Getting Started

Install dependencies:

```bash
uv sync --extra dev
```

Run the development server:

```bash
uv run python -m app.server
```

The API runs at:

```txt
http://127.0.0.1:8000
```

Host, port, and reload behavior can be changed in `.env`:

```txt
HOST=127.0.0.1
PORT=8000
RELOAD=true
```

Use `RELOAD=true` for local development. Use `RELOAD=false` when running without hot reload.

Swagger docs:

```txt
http://127.0.0.1:8000/docs
```

## Tests

```bash
uv run pytest
```

## Dataset validation

Validate the bundled static dataset without starting the API server:

```bash
uv run mlbb-validate-dataset
```

Validate another dataset directory:

```bash
uv run mlbb-validate-dataset --data-dir app/data/static
```

The command exits non-zero and prints validation errors when hero or counter data is invalid.

## API Endpoints

- `GET /health`
- `GET /api/heroes`
- `GET /api/heroes/{hero_id}`
- `GET /api/heroes/{hero_id}/counters`
- `POST /api/counters/analyze-score`
- `POST /api/counters/analyze-detail`

Hero list filters:

- `search`
- `role`
- `lane`
- `page`
- `size`

Example:

```bash
curl "http://127.0.0.1:8000/api/heroes?search=tig&role=tank&lane=roam&page=1&size=10"
```

## Deploy to Vercel

1. Import the GitHub repo in Vercel and choose the **FastAPI** preset.
2. Set **Install Command** to `uv sync` (no `--extra dev` needed in production).
3. Add environment variables in the Vercel dashboard:

| Variable | Required | Example |
|----------|----------|---------|
| `FRONTEND_ORIGIN` | Yes (production) | `https://your-frontend.vercel.app` |
| `AI_PROVIDER` | Yes (for AI scoring) | `openrouter` |
| `<AI_PROVIDER>_API_KEY` | Yes (for AI scoring) | `sk-or-...` |
| `<AI_PROVIDER>_MODEL` | Recommended | `openrouter/free` |
| `AI_TIMEOUT_SECONDS` | Optional | `20` |

**Provider env naming:** set `AI_PROVIDER` to the provider slug in lowercase (for example `openrouter` or `openai`). Build the other AI variables by uppercasing that slug:

- `AI_PROVIDER=openrouter` → `OPENROUTER_API_KEY`, `OPENROUTER_MODEL`
- `AI_PROVIDER=openai` → `OPENAI_API_KEY`, `OPENAI_MODEL`

Optional provider-specific settings (see `.env.example`) follow the same uppercase prefix, such as `OPENROUTER_SERVER_URL`.

`HOST`, `PORT`, and `RELOAD` are for local `uvicorn` only; Vercel does not need them.

The hero dataset lives in `app/data/static/` and is packaged with the FastAPI app so serverless cold starts can validate and load it.

After deploy:

```bash
curl https://<your-api>.vercel.app/health
curl "https://<your-api>.vercel.app/api/heroes?page=1&size=5"
```

Point the frontend at the API with `NEXT_PUBLIC_ANALYZER_API_URL=https://<your-api>.vercel.app`.

## AI scoring

Copy `.env.example` to `.env`, set `AI_PROVIDER`, then set `<AI_PROVIDER>_API_KEY` in uppercase (for OpenRouter: `OPENROUTER_API_KEY` from [OpenRouter keys](https://openrouter.ai/settings/keys)).

```bash
curl -X POST http://127.0.0.1:8000/api/counters/analyze-score \
  -H "Content-Type: application/json" \
  -d '{"targetHeroId":"tigreal"}'

curl -X POST http://127.0.0.1:8000/api/counters/analyze-detail \
  -H "Content-Type: application/json" \
  -d '{"targetHeroId":"tigreal","counterHeroId":"diggie"}'
```

Without a configured provider, both endpoints return an error JSON payload (no fallback scores).
