# Frontend Integration

The frontend should call the Python service instead of importing counter datasets directly once the backend is ready.

## Environment Variable

```txt
NEXT_PUBLIC_ANALYZER_API_URL=http://localhost:8000
```

## Request Flow

1. User selects enemy hero.
2. Frontend sets visible score to `0`.
3. Frontend starts analyzing state.
4. Frontend calls:

```txt
POST ${NEXT_PUBLIC_ANALYZER_API_URL}/api/analyze
```

5. While waiting, score can show a ticker or odometer animation.
6. API returns final `score` and `confidence`.
7. Frontend animates visible score from current ticker value to final score.
8. Frontend displays explanation fields from API response.

## Score Animation

Recommended names for the behavior:

- count-up animation;
- odometer animation;
- number ticker;
- slot-machine ticker.

Implementation idea:

- Initialize every card with `score: 0`.
- During loading, update a temporary display number every 80-120ms.
- Use bounded random values, for example `35-95`, so it feels active.
- When API completes, stop the ticker.
- Animate from the current displayed number to the returned final score.

## Frontend Response Type

```ts
export type AnalyzeResponse = {
  targetHeroId: string;
  source: "ai" | "fallback";
  recommendations: Array<{
    rank: number;
    counterHeroId: string;
    score: number;
    confidence: number;
    summary: string;
    strengths: string[];
    conditions: string[];
    failureCases: string[];
    evidenceIds: string[];
  }>;
};
```

## Failure Handling

If the API is unavailable:

- keep the selected hero visible;
- stop the ticker;
- show score `0`;
- show a concise error state;
- do not invent analysis in the frontend.

