# AI Scoring Contract

AI scoring should produce runtime analysis from reviewed dataset context.

The AI must not invent hero skills, item requirements, patch facts, matchup claims, or failure cases. It may only score and explain using the supplied `reasons`, `counterTypes`, and `proof`.

## Score Meaning

`score` is matchup strength from `0` to `100`.

- `95-100`: hard counter or very direct mechanic counter.
- `85-94`: strong and reliable counter.
- `75-84`: good counter with meaningful conditions.
- `65-74`: situational counter.
- `<65`: weak, incomplete, or too conditional.

## Confidence Meaning

`confidence` is how complete and reliable the supplied evidence is from `0` to `100`.

High context should increase confidence, not automatically increase score. A counter with many failure cases may have high confidence but a lower score.

## Scoring Guidance

Prefer higher scores when:

- proof directly answers the target hero's main threat;
- proof is `primary`;
- proof impact is `high`;
- failure cases are narrow or avoidable;
- the counter works before needing expensive item timing.

Prefer lower scores when:

- proof is mostly item timing;
- proof is mostly positioning or cooldown dependent;
- the counter can fail easily;
- the evidence is generic or only indirectly relevant.

## Prompt Guardrails

Use this as the core developer instruction:

```txt
You are scoring Mobile Legends hero counter recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each counter, return:
- score: 0-100 matchup strength
- confidence: 0-100 evidence confidence
- summary: concise explanation
- strengths: concrete strengths from provided evidence
- conditions: works-best conditions from provided evidence
- failureCases: failure cases from provided evidence
- evidenceIds: proof IDs used

Direct skill interactions and direct sustain or crowd-control counters should score higher than generic damage or item-dependent counters.
More context increases confidence, not necessarily score.
```

## Provider

Providers live under `app/analyzer/providers/`:

- `base.py` — shared `ChatProvider` protocol, JSON parsing, errors
- `openrouter.py` — [OpenRouter Python SDK](https://openrouter.ai/sdk)
- `openai.py` — placeholder for direct OpenAI (not wired yet)

`create_chat_provider()` in `providers/__init__.py` selects the implementation from `AI_PROVIDER`.

The service uses OpenRouter when `AI_PROVIDER=openrouter`.

Environment variables:

- `OPENROUTER_API_KEY`
- `OPENROUTER_MODEL`
- `OPENROUTER_SERVER_URL`
- `OPENROUTER_APP_TITLE`
- `OPENROUTER_HTTP_REFERER`
- `AI_TIMEOUT_SECONDS`

Scoring and detail are split into two API calls:

- `POST /api/counters/analyze-score` scores every counter matchup in one batch.
- `POST /api/counters/analyze-detail` explains one target/counter pair on demand.

## Errors (no fallback)

If the provider is missing, not implemented, or the model call fails, the API returns an error response. The frontend should surface `error.message` and keep using dataset context from `GET /api/heroes/{id}/counters`.

