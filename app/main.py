from contextlib import asynccontextmanager
from typing import AsyncIterator

from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.api import counters, health, heroes
from app.core.config import DATA_DIR, get_settings
from app.data.loader import Dataset
from app.data.validation import validate_dataset


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    validate_dataset(DATA_DIR)
    app.state.dataset = Dataset(DATA_DIR)
    yield


def create_app() -> FastAPI:
    settings = get_settings()
    app = FastAPI(
        title="MLBB Analyzer Service",
        summary="FastAPI service for MLBB counter analysis.",
        description=(
            "Loads and validates the local MLBB hero counter dataset, then returns "
            "counter recommendations for frontend consumption."
        ),
        version="0.1.0",
        docs_url="/docs",
        redoc_url="/redoc",
        openapi_url="/openapi.json",
        openapi_tags=[
            {"name": "health", "description": "Service health endpoints."},
            {
                "name": "heroes",
                "description": "Hero catalog and counter matchup endpoints.",
            },
            {
                "name": "counters",
                "description": "AI scoring and detail endpoints for counter matchups.",
            },
        ],
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.allowed_origins(),
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(health.router)
    app.include_router(heroes.router)
    app.include_router(counters.router)

    @app.get("/", summary="Root")
    def root() -> str:
        return "Hello, World!"

    @app.exception_handler(HTTPException)
    async def http_exception_handler(
        request: Request, exc: HTTPException
    ) -> JSONResponse:
        detail = exc.detail
        if isinstance(detail, dict) and "code" in detail and "message" in detail:
            return JSONResponse(status_code=exc.status_code, content={"error": detail})
        return JSONResponse(
            status_code=exc.status_code,
            content={"error": {"code": "http_error", "message": str(detail)}},
        )

    return app


app = create_app()
