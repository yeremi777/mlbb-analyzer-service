from collections import OrderedDict
from threading import RLock
from time import monotonic
from typing import Any, TypeVar
from weakref import ReferenceType, ref

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
_ResponseT = TypeVar("_ResponseT", bound=BaseModel)
_CacheKey = tuple[Any, ...]
_CacheEntry = tuple[float, ReferenceType[Settings], BaseModel]
_ANALYSIS_CACHE: OrderedDict[_CacheKey, _CacheEntry] = OrderedDict()
_ANALYSIS_CACHE_LOCK = RLock()


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
    for slug in settings.ai_provider_slugs():
        if slug in _NOT_IMPLEMENTED_PROVIDERS:
            raise AnalyzerNotImplementedError(slug)
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


def _provider_cache_fingerprint(settings: Settings) -> tuple[tuple[str, str | None, str | None], ...]:
    fingerprint: list[tuple[str, str | None, str | None]] = []
    for slug in settings.ai_provider_slugs():
        server_url: str | None = None
        if slug == "openrouter":
            server_url = settings.openrouter_server_url
        elif slug == "opencode_zen":
            server_url = settings.opencode_zen_server_url
        fingerprint.append((slug, settings.provider_model(slug), server_url))
    return tuple(fingerprint)


def _analysis_cache_scope(settings: Settings) -> tuple[int, tuple[tuple[str, str | None, str | None], ...]]:
    return (id(settings), _provider_cache_fingerprint(settings))


def _cache_get(key: _CacheKey, settings: Settings, response_type: type[_ResponseT]) -> _ResponseT | None:
    ttl_seconds = settings.ai_analysis_cache_ttl_seconds
    max_entries = settings.ai_analysis_cache_max_entries
    if ttl_seconds <= 0 or max_entries <= 0:
        return None

    now = monotonic()
    with _ANALYSIS_CACHE_LOCK:
        cached = _ANALYSIS_CACHE.get(key)
        if cached is None:
            return None

        expires_at, settings_ref, response = cached
        if expires_at <= now or settings_ref() is not settings:
            _ANALYSIS_CACHE.pop(key, None)
            return None

        _ANALYSIS_CACHE.move_to_end(key)
        if not isinstance(response, response_type):
            return None
        return response.model_copy(deep=True)


def _cache_set(key: _CacheKey, settings: Settings, response: BaseModel) -> None:
    ttl_seconds = settings.ai_analysis_cache_ttl_seconds
    max_entries = settings.ai_analysis_cache_max_entries
    if ttl_seconds <= 0 or max_entries <= 0:
        return

    expires_at = monotonic() + ttl_seconds
    with _ANALYSIS_CACHE_LOCK:
        _ANALYSIS_CACHE[key] = (expires_at, ref(settings), response.model_copy(deep=True))
        _ANALYSIS_CACHE.move_to_end(key)
        while len(_ANALYSIS_CACHE) > max_entries:
            _ANALYSIS_CACHE.popitem(last=False)


def _complete_json(
    provider: ChatProvider,
    settings: Settings,
    messages: list[dict[str, str]],
) -> dict[str, Any]:
    try:
        return provider.complete_json(messages)
    except ProviderError as exc:
        raise _map_provider_error(exc, settings.ai_provider) from exc
    except AnalyzerError:
        raise
    except Exception as exc:
        raise AnalyzerProviderError(str(exc)) from exc


def _run_with_provider(
    settings: Settings,
    messages: list[dict[str, str]],
) -> dict[str, Any]:
    provider: ChatProvider | None = None
    try:
        provider = create_chat_provider(settings)
        return _complete_json(provider, settings, messages)
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

    cache_key = (
        "counter-score",
        target_hero_id,
        language,
        _analysis_cache_scope(settings),
    )
    cached = _cache_get(cache_key, settings, AnalyzeScoresResponse)
    if cached is not None:
        return cached

    matchups = dataset.get_matchups_for_target(target_hero_id)
    target_hero = dataset.heroes_by_id[target_hero_id]
    expected_ids = {matchup.counterHeroId for matchup in matchups}
    messages = build_scoring_messages(target_hero, matchups, dataset.heroes_by_id, language)

    try:
        payload = _run_with_provider(settings, messages)
        items = _validate_scoring_payload(payload, expected_ids)
        recommendations = _rank_score_recommendations(items)
        response = AnalyzeScoresResponse(
            targetHeroId=target_hero_id,
            source="ai",
            recommendations=recommendations,
        )
        _cache_set(cache_key, settings, response)
        return response
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

    cache_key = (
        "counter-detail",
        target_hero_id,
        counter_hero_id,
        language,
        _analysis_cache_scope(settings),
    )
    cached = _cache_get(cache_key, settings, AnalyzeDetailResponse)
    if cached is not None:
        return cached

    matchups = dataset.get_matchups_for_target(target_hero_id)
    matchup = next((row for row in matchups if row.counterHeroId == counter_hero_id), None)
    if matchup is None:
        raise ValueError("counter matchup not found")

    target_hero = dataset.heroes_by_id[target_hero_id]
    counter_hero = dataset.heroes_by_id[counter_hero_id]
    messages = build_detail_messages(target_hero, matchup, counter_hero, language)
    allowed_ids = allowed_evidence_ids(matchup)

    provider: ChatProvider | None = None
    try:
        provider = create_chat_provider(settings)
        payload = _complete_json(provider, settings, messages)
        try:
            parsed = validate_detail_payload(payload, allowed_ids)
        except ValidationError as first_exc:
            repair_messages = build_detail_repair_messages(messages, payload, first_exc, language)
            retry_payload = _complete_json(provider, settings, repair_messages)
            parsed = validate_detail_payload(retry_payload, allowed_ids)
        response = AnalyzeDetailResponse(
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
        _cache_set(cache_key, settings, response)
        return response
    except ValidationError as exc:
        raise AnalyzerProviderError(f"Invalid detail response from model after retry: {exc}") from exc
    except ProviderError as exc:
        raise _map_provider_error(exc, settings.ai_provider) from exc
    except AnalyzerError:
        raise
    except Exception as exc:
        raise AnalyzerProviderError(str(exc)) from exc
    finally:
        if provider is not None:
            provider.close()


def run_synergy_scoring_analysis(
    dataset: Dataset,
    anchor_hero_id: str,
    settings: Settings,
    language: str,
) -> AnalyzeSynergyScoresResponse:
    _ensure_ai_ready(settings)

    cache_key = (
        "synergy-score",
        anchor_hero_id,
        language,
        _analysis_cache_scope(settings),
    )
    cached = _cache_get(cache_key, settings, AnalyzeSynergyScoresResponse)
    if cached is not None:
        return cached

    synergies = dataset.get_synergies_for_anchor(anchor_hero_id)
    anchor_hero = dataset.heroes_by_id[anchor_hero_id]
    expected_ids = {synergy.synergyHeroId for synergy in synergies}
    messages = build_synergy_scoring_messages(anchor_hero, synergies, dataset.heroes_by_id, language)

    try:
        payload = _run_with_provider(settings, messages)
        items = _validate_synergy_scoring_payload(payload, expected_ids)
        recommendations = _rank_synergy_recommendations(items)
        response = AnalyzeSynergyScoresResponse(
            anchorHeroId=anchor_hero_id,
            source="ai",
            recommendations=recommendations,
        )
        _cache_set(cache_key, settings, response)
        return response
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

    cache_key = (
        "synergy-detail",
        anchor_hero_id,
        synergy_hero_id,
        language,
        _analysis_cache_scope(settings),
    )
    cached = _cache_get(cache_key, settings, AnalyzeSynergyDetailResponse)
    if cached is not None:
        return cached

    synergies = dataset.get_synergies_for_anchor(anchor_hero_id)
    synergy = next((row for row in synergies if row.synergyHeroId == synergy_hero_id), None)
    if synergy is None:
        raise ValueError("synergy matchup not found")

    anchor_hero = dataset.heroes_by_id[anchor_hero_id]
    synergy_hero = dataset.heroes_by_id[synergy_hero_id]
    messages = build_synergy_detail_messages(anchor_hero, synergy, synergy_hero, language)
    allowed_ids = allowed_evidence_ids(synergy)

    provider: ChatProvider | None = None
    try:
        provider = create_chat_provider(settings)
        payload = _complete_json(provider, settings, messages)
        try:
            parsed = validate_detail_payload(payload, allowed_ids)
        except ValidationError as first_exc:
            repair_messages = build_detail_repair_messages(messages, payload, first_exc, language)
            retry_payload = _complete_json(provider, settings, repair_messages)
            parsed = validate_detail_payload(retry_payload, allowed_ids)
        response = AnalyzeSynergyDetailResponse(
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
        _cache_set(cache_key, settings, response)
        return response
    except ValidationError as exc:
        raise AnalyzerProviderError(
            f"Invalid synergy detail response from model after retry: {exc}"
        ) from exc
    except ProviderError as exc:
        raise _map_provider_error(exc, settings.ai_provider) from exc
    except AnalyzerError:
        raise
    except Exception as exc:
        raise AnalyzerProviderError(str(exc)) from exc
    finally:
        if provider is not None:
            provider.close()
