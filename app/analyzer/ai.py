import json
from typing import Any

from pydantic import BaseModel, Field, ValidationError

from app.analyzer.errors import (
    AnalyzerError,
    AnalyzerNotConfiguredError,
    AnalyzerNotImplementedError,
    AnalyzerProviderError,
)
from app.analyzer.prompt import build_detail_messages, build_scoring_messages
from app.analyzer.providers import ChatProvider, ProviderError, create_chat_provider
from app.core.config import Settings
from app.data.loader import Dataset
from app.schemas.analysis import (
    AnalyzeDetailResponse,
    AnalyzeScoresResponse,
    ScoreRecommendation,
)
from app.schemas.counter import CounterMatchup

_NOT_IMPLEMENTED_PROVIDERS = frozenset({"openai", "anthropic"})


class _ScoringBatchPayload(BaseModel):
    recommendations: list["_ScoringItem"]


class _ScoringItem(BaseModel):
    counterHeroId: str = Field(min_length=1)
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)


class _DetailPayload(BaseModel):
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)
    summary: str = Field(min_length=1)
    strengths: list[str] = Field(min_length=1)
    conditions: list[str] = Field(default_factory=list)
    failureCases: list[str] = Field(default_factory=list)
    evidenceIds: list[str] = Field(default_factory=list)


_DETAIL_REPAIR_INSTRUCTION = """The previous JSON did not match the required detail response schema.

Return corrected JSON only with exactly these keys:
- score: integer 0-100
- confidence: integer 0-100
- summary: non-empty string
- strengths: non-empty array of strings from the provided reasons/proof
- conditions: array of strings from proof.worksBestWhen
- failureCases: array of strings from proof.failureCases
- evidenceIds: array of proof ids present in the input

Do not add matchup facts outside the provided dataset context."""


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


def _allowed_evidence_ids(matchup: CounterMatchup) -> set[str]:
    return {proof.id for proof in matchup.proof}


def _validate_evidence_ids(evidence_ids: list[str], allowed_ids: set[str]) -> bool:
    if not evidence_ids:
        return True
    return all(evidence_id in allowed_ids for evidence_id in evidence_ids)


def _validate_scoring_payload(
    payload: dict[str, Any],
    expected_counter_ids: set[str],
) -> list[_ScoringItem]:
    parsed = _ScoringBatchPayload.model_validate(payload)
    returned_ids = {item.counterHeroId for item in parsed.recommendations}
    if returned_ids != expected_counter_ids:
        raise AnalyzerProviderError(
            "Scoring response must include every expected counterHeroId once."
        )
    return parsed.recommendations


def _build_detail_repair_messages(
    messages: list[dict[str, str]],
    payload: dict[str, Any],
    exc: ValidationError,
) -> list[dict[str, str]]:
    return [
        *messages,
        {"role": "assistant", "content": json.dumps(payload, ensure_ascii=False)},
        {
            "role": "user",
            "content": (
                f"{_DETAIL_REPAIR_INSTRUCTION}\n\n"
                f"Validation error:\n{exc}"
            ),
        },
    ]


def _validate_detail_payload(
    payload: dict[str, Any],
    allowed_ids: set[str],
) -> _DetailPayload:
    parsed = _DetailPayload.model_validate(payload)
    if not _validate_evidence_ids(parsed.evidenceIds, allowed_ids):
        raise AnalyzerProviderError("Detail response referenced unknown evidence ids.")
    return parsed


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
    allowed_ids = _allowed_evidence_ids(matchup)

    try:
        payload = _run_with_provider(settings, messages)
        try:
            parsed = _validate_detail_payload(payload, allowed_ids)
        except ValidationError as first_exc:
            repair_messages = _build_detail_repair_messages(messages, payload, first_exc)
            retry_payload = _run_with_provider(settings, repair_messages)
            parsed = _validate_detail_payload(retry_payload, allowed_ids)
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
