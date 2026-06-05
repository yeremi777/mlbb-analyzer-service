# API Contract

Counter context (`reasons`, `proof`, hero metadata) comes from `GET /api/heroes/{hero_id}/counters`. AI endpoints only add runtime scores and explanations.

## Analyze Score (fast path)

```http
POST /api/counters/analyze-score
```

### Request

```json
{
  "targetHeroId": "tigreal",
  "language": "en"
}
```

### Response

```json
{
  "targetHeroId": "tigreal",
  "source": "ai",
  "recommendations": [
    {
      "rank": 1,
      "counterHeroId": "diggie",
      "score": 96,
      "confidence": 92
    }
  ]
}
```

### Errors

- `404` — target hero or counter data not found
- `501` — `AI_PROVIDER` is not implemented (for example `openai`)
- `502` — model call failed or returned invalid JSON
- `503` — rate limit storage is unavailable or misconfigured
- `504` — provider is not configured (missing API key) or request timed out
- `429` — rate limit exceeded

Default rate limit:

- analyze-score: 5 requests per browser/IP per 5 hours
- analyze-detail: 15 requests per browser/IP per 5 hours

## Analyze Detail (slow path)

```http
POST /api/counters/analyze-detail
```

### Request

```json
{
  "targetHeroId": "tigreal",
  "counterHeroId": "diggie",
  "language": "en"
}
```

### Response

```json
{
  "targetHeroId": "tigreal",
  "counterHeroId": "diggie",
  "source": "ai",
  "score": 96,
  "confidence": 92,
  "summary": "Diggie directly answers Tigreal's AoE crowd-control engage.",
  "strengths": ["Directly reduces Tigreal's main engage value."],
  "conditions": ["Diggie saves ultimate for Tigreal's real engage."],
  "failureCases": ["Tigreal baits Diggie's ultimate first."],
  "evidenceIds": ["diggie-time-journey-vs-tigreal-engage"]
}
```

### Errors

Same AI error codes as analyze-score, plus:

- `404` `counter_hero_not_found`
- `404` `counter_matchup_not_found`

Rate limited responses include `Retry-After` when the remaining Redis key TTL is available:

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Too many analyze requests. Please try again later."
  }
}
```

## Error Response Shape

```json
{
  "error": {
    "code": "ai_provider_error",
    "message": "OpenRouter returned invalid JSON."
  }
}
```

## Health Check

```http
GET /health
```

```json
{
  "status": "ok"
}
```
