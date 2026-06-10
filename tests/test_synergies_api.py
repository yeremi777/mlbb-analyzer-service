from fastapi.testclient import TestClient

from app.core.config import DATA_DIR, Settings
from app.data.loader import Dataset
from app.main import app


class _FakeRedis:
    def __init__(self) -> None:
        self.counts: dict[str, int] = {}
        self.expirations: dict[str, int] = {}

    def incr(self, key: str) -> int:
        self.counts[key] = self.counts.get(key, 0) + 1
        return self.counts[key]

    def expire(self, key: str, seconds: int) -> None:
        self.expirations[key] = seconds

    def ttl(self, key: str) -> int:
        return self.expirations.get(key, 600)

    def eval(
        self,
        _script: str,
        _numkeys: int,
        key: str,
        max_requests: int,
        window_seconds: int,
    ) -> list[int]:
        count = self.counts.get(key)
        if count is not None and count >= max_requests:
            return [0, count, self.ttl(key)]

        count = self.incr(key)
        if count == 1:
            self.expire(key, window_seconds)
        return [1, count, self.ttl(key)]


class _FakeProvider:
    provider_name = "Fake"

    def __init__(self, payload: dict[str, object]) -> None:
        self.payload = payload
        self.call_count = 0

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, object]:
        self.call_count += 1
        return self.payload

    def close(self) -> None:
        return None


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


def test_cached_analyze_synergy_score_does_not_increment_rate_limit(monkeypatch) -> None:
    fake_redis = _FakeRedis()
    settings = Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
        REDIS_HOST="localhost",
        REDIS_PORT=6379,
        REDIS_DB=0,
        RATE_LIMIT_ENABLED=True,
        RATE_LIMIT_ANALYZE_MAX_REQUESTS=1,
        RATE_LIMIT_ANALYZE_WINDOW_SECONDS=600,
    )
    provider = _FakeProvider(
        {
            "recommendations": [
                {"synergyHeroId": "tigreal", "score": 95, "confidence": 90},
                {"synergyHeroId": "diggie", "score": 88, "confidence": 84},
                {"synergyHeroId": "angela", "score": 85, "confidence": 82},
                {"synergyHeroId": "estes", "score": 80, "confidence": 78},
                {"synergyHeroId": "mathilda", "score": 76, "confidence": 74},
            ]
        }
    )
    monkeypatch.setattr("app.core.rate_limit._redis_client", lambda redis_url: fake_redis)
    monkeypatch.setattr("app.api.synergies.get_settings", lambda: settings)
    monkeypatch.setattr("app.analyzer.ai.create_chat_provider", lambda settings: provider)

    with TestClient(app) as client:
        first_response = client.post(
            "/api/synergies/analyze-score",
            json={"anchorHeroId": "miya"},
        )
        second_response = client.post(
            "/api/synergies/analyze-score",
            json={"anchorHeroId": "miya"},
        )

    assert [first_response.status_code, second_response.status_code] == [200, 200]
    assert provider.call_count == 1
    assert max(fake_redis.counts.values()) == 1


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
