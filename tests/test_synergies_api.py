from fastapi.testclient import TestClient

from app.core.config import DATA_DIR, Settings
from app.data.loader import Dataset
from app.main import app


class _SynergyNoDataDataset:
    def __init__(self, dataset: Dataset, anchor_without_data: str) -> None:
        self._dataset = dataset
        self._anchor_without_data = anchor_without_data

    def __getattr__(self, name: str):
        return getattr(self._dataset, name)

    def get_synergies_for_anchor(self, anchor_hero_id: str):
        if anchor_hero_id == self._anchor_without_data:
            return []
        return self._dataset.get_synergies_for_anchor(anchor_hero_id)


def test_analyze_synergy_score_requires_configured_provider(monkeypatch) -> None:
    monkeypatch.setattr(
        "app.api.synergies.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="",
            RATE_LIMIT_ENABLED=False,
        ),
    )

    with TestClient(app) as client:
        response = client.post("/api/synergies/analyze-score", json={"anchorHeroId": "miya"})

    body = response.json()
    assert response.status_code in {501, 502, 504}
    assert "error" in body
    assert body["error"]["code"] in {
        "ai_provider_not_configured",
        "ai_provider_not_implemented",
        "ai_provider_error",
    }


def test_analyze_synergy_score_unknown_anchor_error_shape() -> None:
    with TestClient(app) as client:
        response = client.post("/api/synergies/analyze-score", json={"anchorHeroId": "unknown"})

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "anchor_hero_not_found",
            "message": "Anchor hero was not found in the dataset.",
        }
    }


def test_analyze_synergy_score_anchor_without_data_error_shape() -> None:
    # Exercise the no-data branch with controlled test state; the bundled dataset
    # currently has curated synergies for every catalog hero.
    with TestClient(app) as client:
        original_dataset = app.state.dataset
        try:
            app.state.dataset = _SynergyNoDataDataset(Dataset(DATA_DIR), "ling")
            response = client.post("/api/synergies/analyze-score", json={"anchorHeroId": "ling"})
        finally:
            app.state.dataset = original_dataset

    assert response.status_code == 404
    assert response.json()["error"]["code"] == "synergy_data_not_found"


def test_analyze_synergy_detail_unknown_matchup_error_shape() -> None:
    # miya is an anchor, but valir is not one of her curated synergy partners.
    with TestClient(app) as client:
        response = client.post(
            "/api/synergies/analyze-detail",
            json={"anchorHeroId": "miya", "synergyHeroId": "valir"},
        )

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "synergy_matchup_not_found",
            "message": "Synergy matchup was not found for the anchor hero.",
        }
    }


def test_analyze_synergy_detail_unknown_synergy_hero_error_shape() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/api/synergies/analyze-detail",
            json={"anchorHeroId": "miya", "synergyHeroId": "unknown"},
        )

    assert response.status_code == 404
    assert response.json()["error"]["code"] == "synergy_hero_not_found"
