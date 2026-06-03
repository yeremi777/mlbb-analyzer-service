# Implementation Plan

## Suggested Stack

- Python 3.12+
- FastAPI
- Uvicorn
- Pydantic v2
- OpenAI Python SDK or another provider SDK
- pytest
- python-dotenv or pydantic-settings

## Suggested Project Structure

```txt
mlbb-analyzer-api/
  app/
    main.py
    api/
      analyze.py
      health.py
    core/
      config.py
    data/
      loader.py
      validation.py
    analyzer/
      deterministic.py
      ai.py
      prompt.py
    schemas/
      hero.py
      counter.py
      analysis.py
  data/
    heroes.json
    counters.json
    counters/
      miya.json
      balmond.json
      saber.json
      alice.json
      nana.json
      tigreal.json
      alucard.json
  tests/
    test_validate_counters.py
    test_analyze_contract.py
  pyproject.toml
  README.md
```

## First Milestone

1. Initialize FastAPI project.
2. Move `heroes.json`, `counters.json`, and `counters/*.json` from the frontend repo into `data/`.
3. Add Pydantic schemas for heroes, counters, proof, and analysis response.
4. Add dataset loader and validation.
5. Add `GET /health`.
6. Add split counter analysis endpoints.
7. Add CORS for the frontend dev URL.

## Second Milestone

1. Add AI provider config.
2. Add AI prompt builder.
3. Add structured output validation.
4. Add retry and timeout handling.
5. Add response fallback when AI fails.
6. Add frontend integration.

## Environment Variables

```txt
AI_PROVIDER=openai
OPENAI_API_KEY=
AI_MODEL=
FRONTEND_ORIGIN=http://localhost:3000
AI_TIMEOUT_SECONDS=20
```

## Minimal Dependencies

```txt
fastapi
uvicorn[standard]
pydantic
pydantic-settings
openai
pytest
python-dotenv
```

Add more dependencies only when needed.
