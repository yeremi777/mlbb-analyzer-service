from fastapi.testclient import TestClient

from app.core.config import Settings
from app.main import app
from app.schemas.analysis import AnalyzeDetailResponse, AnalyzeScoresResponse, ScoreRecommendation


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


def test_analyze_score_requires_configured_provider(monkeypatch) -> None:
    monkeypatch.setattr(
        "app.api.counters.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="",
            RATE_LIMIT_ENABLED=False,
        ),
    )

    with TestClient(app) as client:
        response = client.post("/api/counters/analyze-score", json={"targetHeroId": "tigreal"})

    body = response.json()
    assert response.status_code in {501, 502, 504}
    assert "error" in body
    assert body["error"]["code"] in {
        "ai_provider_not_configured",
        "ai_provider_not_implemented",
        "ai_provider_error",
    }


def test_analyze_score_unknown_target_error_shape() -> None:
    with TestClient(app) as client:
        response = client.post("/api/counters/analyze-score", json={"targetHeroId": "unknown"})

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "target_hero_not_found",
            "message": "Target hero was not found in the dataset.",
        }
    }


def test_analyze_score_rate_limit_sets_cookie_and_blocks_excess_requests(monkeypatch) -> None:
    fake_redis = _FakeRedis()
    monkeypatch.setattr("app.core.rate_limit._redis_client", lambda redis_url: fake_redis)
    monkeypatch.setattr(
        "app.api.counters.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="test-key",
            REDIS_HOST="localhost",
            REDIS_PORT=6379,
            REDIS_DB=0,
            RATE_LIMIT_ENABLED=True,
            RATE_LIMIT_ANALYZE_MAX_REQUESTS=1,
            RATE_LIMIT_ANALYZE_WINDOW_SECONDS=600,
        ),
    )
    monkeypatch.setattr(
        "app.api.counters.run_scoring_analysis",
        lambda dataset, target_hero_id, settings, language: AnalyzeScoresResponse(
            targetHeroId=target_hero_id,
            recommendations=[
                ScoreRecommendation(
                    rank=1,
                    counterHeroId="diggie",
                    score=96,
                    confidence=92,
                )
            ],
        ),
    )

    with TestClient(app) as client:
        first_response = client.post(
            "/api/counters/analyze-score",
            json={"targetHeroId": "tigreal"},
        )
        second_response = client.post(
            "/api/counters/analyze-score",
            json={"targetHeroId": "tigreal"},
        )

    assert first_response.status_code == 200
    assert "mlbb_analyzer_client_id=" in first_response.headers["set-cookie"]
    assert second_response.status_code == 429
    assert second_response.headers["retry-after"] == "600"
    assert second_response.json() == {
        "error": {
            "code": "rate_limit_exceeded",
            "message": "Too many analyze requests. Please try again later.",
        }
    }
    assert max(fake_redis.counts.values()) == 1


def test_analyze_score_rate_limit_does_not_create_new_client_key_when_ip_blocked(
    monkeypatch,
) -> None:
    fake_redis = _FakeRedis()
    monkeypatch.setattr("app.core.rate_limit._redis_client", lambda redis_url: fake_redis)
    monkeypatch.setattr(
        "app.api.counters.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="test-key",
            REDIS_HOST="localhost",
            REDIS_PORT=6379,
            REDIS_DB=0,
            RATE_LIMIT_ENABLED=True,
            RATE_LIMIT_ANALYZE_MAX_REQUESTS=1,
            RATE_LIMIT_ANALYZE_WINDOW_SECONDS=600,
        ),
    )
    monkeypatch.setattr(
        "app.api.counters.run_scoring_analysis",
        lambda dataset, target_hero_id, settings, language: AnalyzeScoresResponse(
            targetHeroId=target_hero_id,
            recommendations=[
                ScoreRecommendation(
                    rank=1,
                    counterHeroId="diggie",
                    score=96,
                    confidence=92,
                )
            ],
        ),
    )

    with TestClient(app) as client:
        first_response = client.post(
            "/api/counters/analyze-score",
            json={"targetHeroId": "tigreal"},
        )

    client_keys_after_first_request = [
        key for key in fake_redis.counts if ":client:" in key
    ]

    with TestClient(app) as incognito_client:
        blocked_response = incognito_client.post(
            "/api/counters/analyze-score",
            json={"targetHeroId": "tigreal"},
        )

    client_keys_after_blocked_request = [
        key for key in fake_redis.counts if ":client:" in key
    ]

    assert first_response.status_code == 200
    assert blocked_response.status_code == 429
    assert len(client_keys_after_first_request) == 1
    assert client_keys_after_blocked_request == client_keys_after_first_request
    assert max(fake_redis.counts.values()) == 1


def test_analyze_detail_rate_limit_uses_detail_multiplier(monkeypatch) -> None:
    fake_redis = _FakeRedis()
    monkeypatch.setattr("app.core.rate_limit._redis_client", lambda redis_url: fake_redis)
    monkeypatch.setattr(
        "app.api.counters.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="test-key",
            REDIS_HOST="localhost",
            REDIS_PORT=6379,
            REDIS_DB=0,
            RATE_LIMIT_ENABLED=True,
            RATE_LIMIT_ANALYZE_MAX_REQUESTS=1,
            RATE_LIMIT_ANALYZE_WINDOW_SECONDS=600,
            RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER=3,
        ),
    )
    monkeypatch.setattr(
        "app.api.counters.run_detail_analysis",
        lambda dataset, target_hero_id, counter_hero_id, settings, language: AnalyzeDetailResponse(
            targetHeroId=target_hero_id,
            counterHeroId=counter_hero_id,
            source="ai",
            score=96,
            confidence=92,
            summary="Diggie answers Tigreal engage.",
            strengths=["Reduces engage value."],
            conditions=[],
            failureCases=[],
            evidenceIds=[],
        ),
    )

    with TestClient(app) as client:
        responses = [
            client.post(
                "/api/counters/analyze-detail",
                json={"targetHeroId": "tigreal", "counterHeroId": "diggie"},
            )
            for _ in range(4)
        ]

    assert [response.status_code for response in responses] == [200, 200, 200, 429]
    assert responses[-1].json()["error"]["code"] == "rate_limit_exceeded"


def test_analyze_detail_rate_limit_reuses_client_cookie_key(monkeypatch) -> None:
    fake_redis = _FakeRedis()
    monkeypatch.setattr("app.core.rate_limit._redis_client", lambda redis_url: fake_redis)
    monkeypatch.setattr(
        "app.api.counters.get_settings",
        lambda: Settings(
            AI_PROVIDER="openrouter",
            OPENROUTER_API_KEY="test-key",
            REDIS_HOST="localhost",
            REDIS_PORT=6379,
            REDIS_DB=0,
            RATE_LIMIT_ENABLED=True,
            RATE_LIMIT_ANALYZE_MAX_REQUESTS=2,
            RATE_LIMIT_ANALYZE_WINDOW_SECONDS=600,
            RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER=3,
        ),
    )
    monkeypatch.setattr(
        "app.api.counters.run_detail_analysis",
        lambda dataset, target_hero_id, counter_hero_id, settings, language: AnalyzeDetailResponse(
            targetHeroId=target_hero_id,
            counterHeroId=counter_hero_id,
            source="ai",
            score=96,
            confidence=92,
            summary="Diggie answers Tigreal engage.",
            strengths=["Reduces engage value."],
            conditions=[],
            failureCases=[],
            evidenceIds=[],
        ),
    )

    with TestClient(app) as client:
        for _ in range(2):
            response = client.post(
                "/api/counters/analyze-detail",
                json={"targetHeroId": "tigreal", "counterHeroId": "diggie"},
            )
            assert response.status_code == 200

    client_keys = [key for key in fake_redis.counts if ":client:" in key]
    detail_client_keys = [key for key in client_keys if ":analyze-detail:" in key]

    assert len(detail_client_keys) == 1
    assert fake_redis.counts[detail_client_keys[0]] == 2


def test_analyze_detail_unknown_matchup_error_shape() -> None:
    with TestClient(app) as client:
        response = client.post(
            "/api/counters/analyze-detail",
            json={"targetHeroId": "tigreal", "counterHeroId": "miya"},
        )

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "counter_matchup_not_found",
            "message": "Counter matchup was not found for the target hero.",
        }
    }
