from unittest.mock import patch

import pytest

from app.analyzer.ai import run_detail_analysis, run_scoring_analysis
from app.analyzer.errors import (
    AnalyzerNotConfiguredError,
    AnalyzerNotImplementedError,
    AnalyzerProviderError,
)
from app.core.config import DATA_DIR, Settings
from app.data.loader import Dataset


def _openrouter_settings() -> Settings:
    return Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
        OPENROUTER_MODEL="openrouter/free",
    )


def test_scoring_analysis_uses_ai_when_provider_returns_valid_json() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    payload = {
        "recommendations": [
            {"counterHeroId": "diggie", "score": 96, "confidence": 92},
            {"counterHeroId": "valir", "score": 88, "confidence": 85},
            {"counterHeroId": "karrie", "score": 80, "confidence": 78},
            {"counterHeroId": "akai", "score": 74, "confidence": 70},
            {"counterHeroId": "lunox", "score": 70, "confidence": 68},
        ]
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.return_value = payload
        response = run_scoring_analysis(dataset, "tigreal", settings, "en")

    assert response.source == "ai"
    assert response.recommendations[0].counterHeroId == "diggie"
    assert response.recommendations[0].score == 96
    assert len(response.recommendations) == 5
    mock_provider.close.assert_called_once()


def test_scoring_analysis_raises_when_provider_fails() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_factory.side_effect = RuntimeError("network down")
        with pytest.raises(AnalyzerProviderError, match="network down"):
            run_scoring_analysis(dataset, "tigreal", settings, "en")


def test_scoring_analysis_raises_when_not_configured() -> None:
    dataset = Dataset(DATA_DIR)
    settings = Settings(AI_PROVIDER="openrouter", OPENROUTER_API_KEY="")
    with pytest.raises(AnalyzerNotConfiguredError):
        run_scoring_analysis(dataset, "tigreal", settings, "en")


def test_scoring_analysis_raises_when_openai_selected() -> None:
    dataset = Dataset(DATA_DIR)
    settings = Settings(AI_PROVIDER="openai", OPENAI_API_KEY="test-key")
    with pytest.raises(AnalyzerNotImplementedError):
        run_scoring_analysis(dataset, "tigreal", settings, "en")


def test_detail_analysis_uses_ai_when_provider_returns_valid_json() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    payload = {
        "score": 96,
        "confidence": 92,
        "summary": "Diggie answers Tigreal's engage window.",
        "strengths": ["Reduces Tigreal engage value."],
        "conditions": ["Save ultimate for the real engage."],
        "failureCases": ["Tigreal baits ultimate first."],
        "evidenceIds": ["diggie-time-journey-vs-tigreal-engage"],
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.return_value = payload
        response = run_detail_analysis(dataset, "tigreal", "diggie", settings, "en")

    assert response.source == "ai"
    assert response.summary == payload["summary"]
    assert response.evidenceIds == payload["evidenceIds"]


def test_detail_analysis_retries_when_detail_payload_is_missing_required_field() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    invalid_payload = {
        "score": 96,
        "confidence": 92,
        "summary": "Diggie answers Tigreal's engage window.",
        "conditions": ["Save ultimate for the real engage."],
        "failureCases": ["Tigreal baits ultimate first."],
        "evidenceIds": ["diggie-time-journey-vs-tigreal-engage"],
    }
    repaired_payload = {
        **invalid_payload,
        "strengths": ["Reduces Tigreal engage value."],
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.side_effect = [invalid_payload, repaired_payload]
        response = run_detail_analysis(dataset, "tigreal", "diggie", settings, "en")

    assert response.source == "ai"
    assert response.strengths == repaired_payload["strengths"]
    assert mock_provider.complete_json.call_count == 2
    repair_messages = mock_provider.complete_json.call_args_list[1].args[0]
    assert "previous JSON did not match" in repair_messages[-1]["content"]


def test_detail_repair_preserves_indonesian_language() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    invalid_payload = {
        "score": 96,
        "confidence": 92,
        "summary": "Diggie menjawab jendela engage Tigreal.",
        "conditions": ["Simpan ultimate untuk engage sebenarnya."],
        "failureCases": ["Tigreal memancing ultimate lebih dulu."],
        "evidenceIds": ["diggie-time-journey-vs-tigreal-engage"],
    }
    repaired_payload = {
        **invalid_payload,
        "strengths": ["Mengurangi nilai engage Tigreal."],
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.side_effect = [invalid_payload, repaired_payload]
        run_detail_analysis(dataset, "tigreal", "diggie", settings, "id")

    repair_messages = mock_provider.complete_json.call_args_list[1].args[0]
    assert "Indonesian" in repair_messages[-1]["content"]


def test_detail_analysis_raises_when_retry_payload_is_still_invalid() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    invalid_payload = {
        "score": 96,
        "confidence": 92,
        "summary": "Diggie answers Tigreal's engage window.",
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.side_effect = [invalid_payload, invalid_payload]

        with pytest.raises(AnalyzerProviderError, match="after retry"):
            run_detail_analysis(dataset, "tigreal", "diggie", settings, "en")

    assert mock_provider.complete_json.call_count == 2
