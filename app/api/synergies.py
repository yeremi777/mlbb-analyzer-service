from fastapi import APIRouter, HTTPException, Request, Response

from app.analyzer.ai import (
    has_cached_synergy_detail_analysis,
    has_cached_synergy_score_analysis,
    run_synergy_detail_analysis,
    run_synergy_scoring_analysis,
)
from app.analyzer.errors import (
    AnalyzerError,
    AnalyzerNotConfiguredError,
    AnalyzerNotImplementedError,
    AnalyzerProviderError,
)
from app.core.config import get_settings
from app.core.rate_limit import enforce_analyze_rate_limit
from app.schemas.analysis import (
    AnalyzeSynergyDetailRequest,
    AnalyzeSynergyDetailResponse,
    AnalyzeSynergyScoresRequest,
    AnalyzeSynergyScoresResponse,
)
from app.schemas.openapi import SYNERGY_ANALYZE_DETAIL_RESPONSES, SYNERGY_ANALYZE_SCORE_RESPONSES

router = APIRouter(prefix="/api/synergies", tags=["synergies"])


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


def _require_anchor_hero(request: Request, anchor_hero_id: str) -> None:
    if anchor_hero_id not in request.app.state.dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "anchor_hero_not_found",
                "message": "Anchor hero was not found in the dataset.",
            },
        )


def _require_synergies(request: Request, anchor_hero_id: str) -> None:
    synergies = request.app.state.dataset.get_synergies_for_anchor(anchor_hero_id)
    if not synergies:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "synergy_data_not_found",
                "message": "Synergy data was not found for the anchor hero.",
            },
        )


@router.post(
    "/analyze-score",
    response_model=AnalyzeSynergyScoresResponse,
    summary="Score all synergy pairings for one anchor hero",
    description=(
        "Returns AI-produced score and confidence for every synergy pairing of the anchor hero. "
        "Returns an error if the configured AI provider fails or is unavailable."
    ),
    responses=SYNERGY_ANALYZE_SCORE_RESPONSES,
)
def analyze_score(
    payload: AnalyzeSynergyScoresRequest,
    request: Request,
    response: Response,
) -> AnalyzeSynergyScoresResponse:
    _require_anchor_hero(request, payload.anchorHeroId)
    _require_synergies(request, payload.anchorHeroId)
    settings = get_settings()
    if not has_cached_synergy_score_analysis(payload.anchorHeroId, settings, payload.language):
        enforce_analyze_rate_limit(request, response, settings, "analyze-synergy-score")
    try:
        return run_synergy_scoring_analysis(
            request.app.state.dataset,
            payload.anchorHeroId,
            settings,
            payload.language,
        )
    except AnalyzerError as exc:
        _raise_analyzer_error(exc)


@router.post(
    "/analyze-detail",
    response_model=AnalyzeSynergyDetailResponse,
    summary="Explain one synergy pairing in detail",
    description=(
        "Returns strengths, conditions, failure cases, and summary for one anchor/synergy pair. "
        "Returns an error if the configured AI provider fails or is unavailable."
    ),
    responses=SYNERGY_ANALYZE_DETAIL_RESPONSES,
)
def analyze_detail(
    payload: AnalyzeSynergyDetailRequest,
    request: Request,
    response: Response,
) -> AnalyzeSynergyDetailResponse:
    dataset = request.app.state.dataset
    _require_anchor_hero(request, payload.anchorHeroId)

    if payload.synergyHeroId not in dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "synergy_hero_not_found",
                "message": "Synergy hero was not found in the dataset.",
            },
        )

    synergies = dataset.get_synergies_for_anchor(payload.anchorHeroId)
    if not any(synergy.synergyHeroId == payload.synergyHeroId for synergy in synergies):
        raise HTTPException(
            status_code=404,
            detail={
                "code": "synergy_matchup_not_found",
                "message": "Synergy matchup was not found for the anchor hero.",
            },
        )

    settings = get_settings()
    if not has_cached_synergy_detail_analysis(
        payload.anchorHeroId,
        payload.synergyHeroId,
        settings,
        payload.language,
    ):
        enforce_analyze_rate_limit(request, response, settings, "analyze-synergy-detail")
    try:
        return run_synergy_detail_analysis(
            dataset,
            payload.anchorHeroId,
            payload.synergyHeroId,
            settings,
            payload.language,
        )
    except AnalyzerError as exc:
        _raise_analyzer_error(exc)
