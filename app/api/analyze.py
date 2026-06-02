from fastapi import APIRouter, HTTPException, Request

from app.analyzer.deterministic import build_fallback_response
from app.schemas.analysis import AnalyzeRequest, AnalyzeResponse, ErrorResponse

router = APIRouter(prefix="/api", tags=["analysis"])


@router.post(
    "/analyze",
    response_model=AnalyzeResponse,
    summary="Analyze counter recommendations",
    description=(
        "Returns ranked counter recommendations for one target hero. "
        "The current milestone uses deterministic fallback output with score and confidence set to 0."
    ),
    responses={
        404: {
            "model": ErrorResponse,
            "description": "Target hero or counter data was not found.",
        }
    },
)
def analyze(payload: AnalyzeRequest, request: Request) -> AnalyzeResponse:
    dataset = request.app.state.dataset

    if payload.targetHeroId not in dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "target_hero_not_found",
                "message": "Target hero was not found in the dataset.",
            },
        )

    matchups = dataset.get_matchups_for_target(payload.targetHeroId)
    if not matchups:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "counter_data_not_found",
                "message": "Counter data was not found for the target hero.",
            },
        )

    return build_fallback_response(payload.targetHeroId, matchups, payload.limit)
