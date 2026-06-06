from typing import Any

from pydantic import BaseModel, Field, ValidationError

from app.analyzer.detail import (
    allowed_evidence_ids,
    build_detail_repair_messages,
    validate_detail_payload,
)
from app.analyzer.errors import (
    AnalyzerError,
    AnalyzerNotConfiguredError,
    AnalyzerNotImplementedError,
    AnalyzerProviderError,
)
from app.analyzer.prompt import (
    build_detail_messages,
    build_scoring_messages,
    build_synergy_detail_messages,
    build_synergy_scoring_messages,
)
from app.analyzer.providers import ChatProvider, ProviderError, create_chat_provider
from app.core.config import Settings
from app.data.loader import Dataset
from app.schemas.analysis import (
    AnalyzeDetailResponse,
    AnalyzeScoresResponse,
    AnalyzeSynergyDetailResponse,
    AnalyzeSynergyScoresResponse,
    ScoreRecommendation,
    SynergyScoreRecommendation,
)

_NOT_IMPLEMENTED_PROVIDERS = frozenset({"openai", "anthropic"})


class _ScoringBatchPayload(BaseModel):
    recommendations: list["_ScoringItem"]


class _ScoringItem(BaseModel):
    counterHeroId: str = Field(min_length=1)
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)


class _SynergyScoringBatchPayload(BaseModel):
    recommendations: list["_SynergyScoringItem"]


class _SynergyScoringItem(BaseModel):
    synergyHeroId: str = Field(min_length=1)
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)


def _ensure_ai_ready(settings: Settings) -> None:
    slug = settings.ai_provider.strip().casefold()
    if slug in _NOT_IMPLEMENTED_PROVIDERS:
        raise AnalyzerNotImplementedError(settings.ai_provider)
    if not settings.ai_enabled():
        raise AnalyzerNotConfiguredError(settings.ai_provider)


def _map_provider_error(exc: ProviderError, provider: str) -> AnalyzerError:
    message = str(exc)
    if "not implemented" in message.casefold():
        return AnalyzerNotImplementedError(provider)
    if "not configured" in message.casefold():
        return AnalyzerNotConfiguredError(provider)
    return AnalyzerProviderError(message)


def _rank_score_recommendations(
    items: list[_ScoringItem],
) -> list[ScoreRecommendation]:
    ordered = sorted(items, key=lambda item: (-item.score, -item.confidence, item.counterHeroId))
    return [
        ScoreRecommendation(
            rank=rank,
            counterHeroId=item.counterHeroId,
            score=item.score,
            confidence=item.confidence,
        )
        for rank, item in enumerate(ordered, start=1)
    ]


def _validate_scoring_payload(
    payload: dict[str, Any],
    expected_counter_ids: set[str],
) -> list[_ScoringItem]:
    parsed = _ScoringBatchPayload.model_validate(payload)
    returned_ids = [item.counterHeroId for item in parsed.recommendations]
    if len(returned_ids) != len(expected_counter_ids) or set(returned_ids) != expected_counter_ids:
        raise AnalyzerProviderError(
            "Scoring response must include every expected counterHeroId once."
        )
    return parsed.recommendations


def _rank_synergy_recommendations(
    items: list[_SynergyScoringItem],
) -> list[SynergyScoreRecommendation]:
    ordered = sorted(items, key=lambda item: (-item.score, -item.confidence, item.synergyHeroId))
    return [
        SynergyScoreRecommendation(
            rank=rank,
            synergyHeroId=item.synergyHeroId,
            score=item.score,
            confidence=item.confidence,
        )
        for rank, item in enumerate(ordered, start=1)
    ]


def _validate_synergy_scoring_payload(
    payload: dict[str, Any],
    expected_synergy_ids: set[str],
) -> list[_SynergyScoringItem]:
    parsed = _SynergyScoringBatchPayload.model_validate(payload)
    returned_ids = [item.synergyHeroId for item in parsed.recommendations]
    if len(returned_ids) != len(expected_synergy_ids) or set(returned_ids) != expected_synergy_ids:
        raise AnalyzerProviderError(
            "Synergy scoring response must include every expected synergyHeroId once."
        )
    return parsed.recommendations


def _run_with_provider(
    settings: Settings,
    messages: list[dict[str, str]],
) -> dict[str, Any]:
    provider: ChatProvider | None = None
    try:
        provider = create_chat_provider(settings)
        return provider.complete_json(messages)
    except ProviderError as exc:
        raise _map_provider_error(exc, settings.ai_provider) from exc
    except AnalyzerError:
        raise
    except Exception as exc:
        raise AnalyzerProviderError(str(exc)) from exc
    finally:
        if provider is not None:
            provider.close()


def run_scoring_analysis(
    dataset: Dataset,
    target_hero_id: str,
    settings: Settings,
    language: str,
) -> AnalyzeScoresResponse:
    _ensure_ai_ready(settings)

    matchups = dataset.get_matchups_for_target(target_hero_id)
    target_hero = dataset.heroes_by_id[target_hero_id]
    expected_ids = {matchup.counterHeroId for matchup in matchups}
    messages = build_scoring_messages(target_hero, matchups, dataset.heroes_by_id, language)

    try:
        payload = _run_with_provider(settings, messages)
        items = _validate_scoring_payload(payload, expected_ids)
        recommendations = _rank_score_recommendations(items)
        return AnalyzeScoresResponse(
            targetHeroId=target_hero_id,
            source="ai",
            recommendations=recommendations,
        )
    except ValidationError as exc:
        raise AnalyzerProviderError(f"Invalid scoring response from model: {exc}") from exc


def run_detail_analysis(
    dataset: Dataset,
    target_hero_id: str,
    counter_hero_id: str,
    settings: Settings,
    language: str,
) -> AnalyzeDetailResponse:
    _ensure_ai_ready(settings)

    matchups = dataset.get_matchups_for_target(target_hero_id)
    matchup = next((row for row in matchups if row.counterHeroId == counter_hero_id), None)
    if matchup is None:
        raise ValueError("counter matchup not found")

    target_hero = dataset.heroes_by_id[target_hero_id]
    counter_hero = dataset.heroes_by_id[counter_hero_id]
    messages = build_detail_messages(target_hero, matchup, counter_hero, language)
    allowed_ids = allowed_evidence_ids(matchup)

    try:
        payload = _run_with_provider(settings, messages)
        try:
            parsed = validate_detail_payload(payload, allowed_ids)
        except ValidationError as first_exc:
            repair_messages = build_detail_repair_messages(messages, payload, first_exc, language)
            retry_payload = _run_with_provider(settings, repair_messages)
            parsed = validate_detail_payload(retry_payload, allowed_ids)
        return AnalyzeDetailResponse(
            targetHeroId=target_hero_id,
            counterHeroId=counter_hero_id,
            source="ai",
            score=parsed.score,
            confidence=parsed.confidence,
            summary=parsed.summary,
            strengths=parsed.strengths,
            conditions=parsed.conditions,
            failureCases=parsed.failureCases,
            evidenceIds=parsed.evidenceIds,
        )
    except ValidationError as exc:
        raise AnalyzerProviderError(f"Invalid detail response from model after retry: {exc}") from exc


def run_synergy_scoring_analysis(
    dataset: Dataset,
    anchor_hero_id: str,
    settings: Settings,
    language: str,
) -> AnalyzeSynergyScoresResponse:
    _ensure_ai_ready(settings)

    synergies = dataset.get_synergies_for_anchor(anchor_hero_id)
    anchor_hero = dataset.heroes_by_id[anchor_hero_id]
    expected_ids = {synergy.synergyHeroId for synergy in synergies}
    messages = build_synergy_scoring_messages(anchor_hero, synergies, dataset.heroes_by_id, language)

    try:
        payload = _run_with_provider(settings, messages)
        items = _validate_synergy_scoring_payload(payload, expected_ids)
        recommendations = _rank_synergy_recommendations(items)
        return AnalyzeSynergyScoresResponse(
            anchorHeroId=anchor_hero_id,
            source="ai",
            recommendations=recommendations,
        )
    except ValidationError as exc:
        raise AnalyzerProviderError(f"Invalid synergy scoring response from model: {exc}") from exc


def run_synergy_detail_analysis(
    dataset: Dataset,
    anchor_hero_id: str,
    synergy_hero_id: str,
    settings: Settings,
    language: str,
) -> AnalyzeSynergyDetailResponse:
    _ensure_ai_ready(settings)

    synergies = dataset.get_synergies_for_anchor(anchor_hero_id)
    synergy = next((row for row in synergies if row.synergyHeroId == synergy_hero_id), None)
    if synergy is None:
        raise ValueError("synergy matchup not found")

    anchor_hero = dataset.heroes_by_id[anchor_hero_id]
    synergy_hero = dataset.heroes_by_id[synergy_hero_id]
    messages = build_synergy_detail_messages(anchor_hero, synergy, synergy_hero, language)
    allowed_ids = allowed_evidence_ids(synergy)

    try:
        payload = _run_with_provider(settings, messages)
        try:
            parsed = validate_detail_payload(payload, allowed_ids)
        except ValidationError as first_exc:
            repair_messages = build_detail_repair_messages(messages, payload, first_exc, language)
            retry_payload = _run_with_provider(settings, repair_messages)
            parsed = validate_detail_payload(retry_payload, allowed_ids)
        return AnalyzeSynergyDetailResponse(
            anchorHeroId=anchor_hero_id,
            synergyHeroId=synergy_hero_id,
            source="ai",
            score=parsed.score,
            confidence=parsed.confidence,
            summary=parsed.summary,
            strengths=parsed.strengths,
            conditions=parsed.conditions,
            failureCases=parsed.failureCases,
            evidenceIds=parsed.evidenceIds,
        )
    except ValidationError as exc:
        raise AnalyzerProviderError(
            f"Invalid synergy detail response from model after retry: {exc}"
        ) from exc
