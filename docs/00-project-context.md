# Python Analyzer Project Context

The Python analyzer service is a separate backend project for `mlbb-analyzer`.

The frontend Next.js app should focus on browser UI, hero selection, reveal animation, and rendering analysis results. The Python service should focus on dataset ownership, deterministic validation, AI-assisted scoring, and explanation output.

## Goal

Provide an API that receives a selected enemy hero and returns ranked counter recommendations with AI-produced scores and readable explanations.

## Initial Scope

- FastAPI HTTP service.
- Static JSON dataset owned by the Python project.
- One target hero analysis.
- AI scoring from reviewed dataset context.
- Structured JSON response for the frontend.
- No database.
- No authentication.
- No scraping.
- No player profile or match history analysis.

## First Ready Heroes

Start with these target hero datasets:

- `miya`
- `balmond`
- `saber`
- `alice`
- `nana`
- `tigreal`
- `alucard`

Other heroes can exist as counter heroes, but only these seven target heroes are initially ready for analysis.

## Service Responsibilities

- Load hero and counter datasets.
- Validate dataset structure at startup or through a validation command.
- Build AI prompts from reviewed context only.
- Produce `score` at runtime, starting from static evidence rather than static score fields.
- Return deterministic fallback output if AI is disabled or unavailable.

## Frontend Responsibilities

- Render hero selector and result cards.
- Show score default value `0` before analysis completes.
- Show loading or number ticker animation while the API is analyzing.
- Render final `score`, `summary`, conditions, and failure cases returned by the Python service.
- Avoid duplicating AI scoring logic in the frontend.

