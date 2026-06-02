from fastapi import APIRouter

router = APIRouter(tags=["health"])


@router.get(
    "/health",
    summary="Health check",
    description="Returns a lightweight service health status.",
)
def health() -> dict[str, str]:
    return {"status": "ok"}
