from fastapi.testclient import TestClient

from app.main import app


def test_analyze_score_requires_configured_provider() -> None:
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


def test_legacy_analyze_route_removed() -> None:
    with TestClient(app) as client:
        response = client.post("/api/analyze", json={"targetHeroId": "tigreal"})

    assert response.status_code == 404
