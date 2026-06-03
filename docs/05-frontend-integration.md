# Frontend Integration

The frontend should call the Python service instead of importing counter datasets directly.

## Environment Variable

```txt
NEXT_PUBLIC_ANALYZER_API_URL=http://localhost:8000
```

## Request Flow

1. User selects enemy hero.
2. Load static matchup context:

```txt
GET ${NEXT_PUBLIC_ANALYZER_API_URL}/api/heroes/{heroId}/counters
```

3. Frontend sets visible score to `0` on every counter card.
4. Frontend starts analyzing state.
5. Frontend calls the fast scores endpoint:

```txt
POST ${NEXT_PUBLIC_ANALYZER_API_URL}/api/counters/analyze-score
```

6. While waiting, score can show a ticker or odometer animation.
7. API returns final `score` and `confidence` for every counter matchup.
8. Frontend sorts by `score`, runs reveal animation, and animates each card to its final score.
9. When the user opens one counter card, call:

```txt
POST ${NEXT_PUBLIC_ANALYZER_API_URL}/api/counters/analyze-detail
```

10. Render `summary`, `strengths`, `conditions`, and `failureCases` from the detail response.

## Score Animation

- Initialize every card with `score: 0`.
- During loading, update a temporary display number every 80-120ms.
- When the scores API completes, stop the ticker and animate to the returned final score.

## Frontend Response Types

```ts
export type AnalyzeScoresResponse = {
  targetHeroId: string;
  source: "ai";
  recommendations: Array<{
    rank: number;
    counterHeroId: string;
    score: number;
    confidence: number;
  }>;
};

export type AnalyzeDetailResponse = {
  targetHeroId: string;
  counterHeroId: string;
  source: "ai";
  score: number;
  confidence: number;
  summary: string;
  strengths: string[];
  conditions: string[];
  failureCases: string[];
  evidenceIds: string[];
};
```

## Failure Handling

If the scores API fails:

- keep the selected hero visible;
- stop the ticker;
- show the API `error.message` (do not invent scores);
- keep showing dataset context from `GET /api/heroes/{id}/counters` if that call succeeded earlier.

If the detail API fails after scores loaded:

- keep card scores visible;
- show the API error in the detail panel;
- do not invent explanation text in the frontend.
