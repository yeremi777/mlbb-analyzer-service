from fastapi import APIRouter, HTTPException, Request

from app.analyzer.ai import run_detail_analysis, run_scoring_analysis
from app.analyzer.errors import (
    AnalyzerError,
    AnalyzerNotConfiguredError,
    AnalyzerNotImplementedError,
    AnalyzerProviderError,
)
from app.core.config import get_settings
from app.schemas.analysis import (
    AnalyzeDetailRequest,
    AnalyzeDetailResponse,
    AnalyzeScoresRequest,
    AnalyzeScoresResponse,
)
from app.schemas.openapi import COUNTER_ANALYZE_DETAIL_RESPONSES, COUNTER_ANALYZE_SCORE_RESPONSES

router = APIRouter(prefix="/api/counters", tags=["counters"])


def _raise_analyzer_error(exc: AnalyzerError) -> None:
    status_code = 504
    if isinstance(exc, AnalyzerNotImplementedError):
        status_code = 501
    elif isinstance(exc, AnalyzerProviderError):
        status_code = 502
    elif isinstance(exc, AnalyzerNotConfiguredError):
        status_code = 504

    raise HTTPException(
        status_code=status_code,
        detail={"code": exc.code, "message": exc.message},
    )


def _require_target_hero(request: Request, target_hero_id: str) -> None:
    if target_hero_id not in request.app.state.dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "target_hero_not_found",
                "message": "Target hero was not found in the dataset.",
            },
        )


def _require_matchups(request: Request, target_hero_id: str) -> None:
    matchups = request.app.state.dataset.get_matchups_for_target(target_hero_id)
    if not matchups:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "counter_data_not_found",
                "message": "Counter data was not found for the target hero.",
            },
        )


@router.post(
    "/analyze-score",
    response_model=AnalyzeScoresResponse,
    summary="Score all counter matchups for one target hero",
    description=(
        "Returns AI-produced score and confidence for every counter matchup of the target hero. "
        "Returns an error if the configured AI provider fails or is unavailable."
    ),
    responses=COUNTER_ANALYZE_SCORE_RESPONSES,
)
def analyze_score(payload: AnalyzeScoresRequest, request: Request) -> AnalyzeScoresResponse:
    _require_target_hero(request, payload.targetHeroId)
    _require_matchups(request, payload.targetHeroId)
    settings = get_settings()
    try:
        return run_scoring_analysis(
            request.app.state.dataset,
            payload.targetHeroId,
            settings,
            payload.language,
        )
    except AnalyzerError as exc:
        _raise_analyzer_error(exc)


@router.post(
    "/analyze-detail",
    response_model=AnalyzeDetailResponse,
    summary="Explain one counter matchup in detail",
    description=(
        "Returns strengths, conditions, failure cases, and summary for one target/counter pair. "
        "Returns an error if the configured AI provider fails or is unavailable."
    ),
    responses=COUNTER_ANALYZE_DETAIL_RESPONSES,
)
def analyze_detail(payload: AnalyzeDetailRequest, request: Request) -> AnalyzeDetailResponse:
    dataset = request.app.state.dataset
    _require_target_hero(request, payload.targetHeroId)

    if payload.counterHeroId not in dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "counter_hero_not_found",
                "message": "Counter hero was not found in the dataset.",
            },
        )

    matchups = dataset.get_matchups_for_target(payload.targetHeroId)
    if not any(matchup.counterHeroId == payload.counterHeroId for matchup in matchups):
        raise HTTPException(
            status_code=404,
            detail={
                "code": "counter_matchup_not_found",
                "message": "Counter matchup was not found for the target hero.",
            },
        )

    settings = get_settings()
    try:
        return run_detail_analysis(
            dataset,
            payload.targetHeroId,
            payload.counterHeroId,
            settings,
            payload.language,
        )
    except AnalyzerError as exc:
        _raise_analyzer_error(exc)
