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
uv run uvicorn app.main:app --reload
```

The API runs at:

```txt
http://127.0.0.1:8000
```

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

## Status

`POST /api/analyze` currently returns deterministic fallback recommendations. AI scoring will be implemented later.
