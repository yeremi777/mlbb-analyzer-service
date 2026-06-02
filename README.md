# MLBB Analyzer Service

FastAPI backend service for an MLBB counter-pick analyzer.

The service loads a local hero and counter dataset, validates it on startup, and exposes API endpoints for the frontend hero selector and counter recommendation flow.

## Features

- Hero list API with search, role, lane, and pagination filters
- Hero detail API
- Hero counter matchup API
- Dataset validation at startup
- Swagger/OpenAPI documentation
- Placeholder analyze endpoint for future AI scoring

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

## API Endpoints

- `GET /health`
- `GET /api/heroes`
- `GET /api/heroes/{hero_id}`
- `GET /api/heroes/{hero_id}/counters`
- `POST /api/analyze`

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
| `OPENAI_API_KEY` | No (until AI scoring ships) | `sk-...` |

`HOST`, `PORT`, and `RELOAD` are for local `uvicorn` only; Vercel does not need them.

The hero dataset lives in `app/data/static/` and is packaged with the FastAPI app so serverless cold starts can validate and load it.

After deploy:

```bash
curl https://<your-api>.vercel.app/health
curl "https://<your-api>.vercel.app/api/heroes?page=1&size=5"
```

Point the frontend at the API with `NEXT_PUBLIC_ANALYZER_API_URL=https://<your-api>.vercel.app`.

## Status

`POST /api/analyze` currently returns deterministic fallback recommendations. AI scoring will be implemented later.
