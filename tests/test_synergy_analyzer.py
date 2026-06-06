from unittest.mock import patch

from app.analyzer.ai import run_synergy_detail_analysis, run_synergy_scoring_analysis
from app.core.config import DATA_DIR, Settings
from app.data.loader import Dataset


def _openrouter_settings() -> Settings:
    return Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
        OPENROUTER_MODEL="openrouter/free",
    )


def test_dataset_loads_and_indexes_synergies() -> None:
    dataset = Dataset(DATA_DIR)
    miya_synergies = dataset.get_synergies_for_anchor("miya")
    assert len(miya_synergies) == 5
    assert {s.synergyHeroId for s in miya_synergies} == {
        "estes",
        "angela",
        "mathilda",
        "franco",
        "tigreal",
    }
    # heroes without curated synergy data return nothing
    assert dataset.get_synergies_for_anchor("layla") == []


def test_synergy_scoring_analysis_uses_ai_when_provider_returns_valid_json() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    payload = {
        "recommendations": [
            {"synergyHeroId": "tigreal", "score": 95, "confidence": 90},
            {"synergyHeroId": "franco", "score": 88, "confidence": 84},
            {"synergyHeroId": "angela", "score": 85, "confidence": 82},
            {"synergyHeroId": "estes", "score": 80, "confidence": 78},
            {"synergyHeroId": "mathilda", "score": 76, "confidence": 74},
        ]
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.return_value = payload
        response = run_synergy_scoring_analysis(dataset, "miya", settings, "en")

    assert response.anchorHeroId == "miya"
    assert response.source == "ai"
    assert [rec.rank for rec in response.recommendations] == [1, 2, 3, 4, 5]
    assert response.recommendations[0].synergyHeroId == "tigreal"


def test_synergy_detail_analysis_uses_ai_when_provider_returns_valid_json() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    payload = {
        "score": 92,
        "confidence": 88,
        "summary": "Tigreal groups enemies so Pharsa lands her AoE ultimate.",
        "strengths": ["Tigreal's lockdown guarantees Pharsa's bombardment lands."],
        "conditions": ["Tigreal lands Implosion on a cluster."],
        "failureCases": ["Enemies are spread out."],
        "evidenceIds": ["pharsa-feathered-air-strike-with-tigreal-grouped-aoe"],
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.return_value = payload
        response = run_synergy_detail_analysis(dataset, "tigreal", "pharsa", settings, "en")

    assert response.anchorHeroId == "tigreal"
    assert response.synergyHeroId == "pharsa"
    assert response.source == "ai"
    assert response.summary == payload["summary"]
    assert response.evidenceIds == payload["evidenceIds"]


def test_synergy_detail_repair_preserves_indonesian_language() -> None:
    dataset = Dataset(DATA_DIR)
    settings = _openrouter_settings()
    invalid_payload = {
        "score": 92,
        "confidence": 88,
        "summary": "Tigreal mengelompokkan musuh agar Pharsa mendaratkan ultimate-nya.",
        "conditions": ["Tigreal mengenai Implosion pada kelompok musuh."],
        "failureCases": ["Musuh tersebar."],
        "evidenceIds": ["pharsa-feathered-air-strike-with-tigreal-grouped-aoe"],
    }
    repaired_payload = {
        **invalid_payload,
        "strengths": ["Kuncian Tigreal menjamin bombardir Pharsa mengenai sasaran."],
    }

    with patch("app.analyzer.ai.create_chat_provider") as mock_factory:
        mock_provider = mock_factory.return_value
        mock_provider.complete_json.side_effect = [invalid_payload, repaired_payload]
        run_synergy_detail_analysis(dataset, "tigreal", "pharsa", settings, "id")

    repair_messages = mock_provider.complete_json.call_args_list[1].args[0]
    assert "Indonesian" in repair_messages[-1]["content"]
