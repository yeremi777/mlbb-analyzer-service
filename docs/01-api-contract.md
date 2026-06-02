# API Contract

## Analyze One Target Hero

```http
POST /api/analyze
```

### Request

```json
{
  "targetHeroId": "tigreal",
  "limit": 5,
  "language": "en"
}
```

Fields:

- `targetHeroId`: required hero UID.
- `limit`: optional number of recommendations to return. Default can be `5`.
- `language`: optional output language. Start with `en`; later support `id`.

The frontend should not send the full dataset. The Python service owns and loads the dataset.

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
      "confidence": 92,
      "summary": "Diggie directly answers Tigreal's AoE crowd-control engage with team cleanse and control immunity.",
      "strengths": [
        "Directly reduces Tigreal's main engage value.",
        "Protects multiple allies during the setup window."
      ],
      "conditions": [
        "Diggie saves ultimate for Tigreal's real engage.",
        "Diggie stays close enough to the teammates Tigreal wants to catch."
      ],
      "failureCases": [
        "Tigreal baits Diggie's ultimate first.",
        "Tigreal catches Diggie away from the team."
      ],
      "evidenceIds": [
        "diggie-time-journey-vs-tigreal-engage"
      ]
    }
  ]
}
```

### Error Response

```json
{
  "error": {
    "code": "target_hero_not_found",
    "message": "Target hero was not found in the dataset."
  }
}
```

Suggested status codes:

- `200`: analysis completed.
- `400`: invalid request.
- `404`: target hero not found or no counter data.
- `500`: internal service error.
- `503`: AI provider unavailable when fallback is disabled.

## Health Check

```http
GET /health
```

```json
{
  "status": "ok"
}
```

## Dataset Summary

```http
GET /api/dataset/summary
```

Useful during development.

```json
{
  "targetHeroCount": 7,
  "counterMatchupCount": 33,
  "readyTargets": ["miya", "balmond", "saber", "alice", "nana", "tigreal", "alucard"]
}
```

