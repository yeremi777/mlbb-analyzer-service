# Repository Linking Plan

The frontend and Python analyzer can be linked later without merging them into one repository.

## Recommended Setup

```txt
workspace/
  mlbb-analyzer/       # Next.js frontend
  mlbb-analyzer-api/   # FastAPI analyzer
```

Run locally:

```txt
Frontend: http://localhost:3000
API:      http://localhost:8000
```

Frontend environment:

```txt
NEXT_PUBLIC_ANALYZER_API_URL=http://localhost:8000
```

Python CORS allowed origin:

```txt
FRONTEND_ORIGIN=http://localhost:3000
```

## Dataset Migration

Move these files from the frontend repo into the Python repo:

```txt
public/data/heroes.json          -> data/heroes.json
public/data/counters.json        -> data/counters.json
public/data/counters/*.json      -> data/counters/*.json
```

After migration, the frontend should stop importing static counters directly and rely on the Python API response.

Hero display data can remain in the frontend temporarily if needed for UI portraits and selector behavior. Later, the Python service can expose hero metadata through an endpoint.

