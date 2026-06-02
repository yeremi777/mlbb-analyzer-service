from fastapi.testclient import TestClient

from app.main import app


def test_health() -> None:
    with TestClient(app) as client:
        response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_analyze_returns_fallback_recommendations() -> None:
    with TestClient(app) as client:
        response = client.post("/api/analyze", json={"targetHeroId": "tigreal", "limit": 2})

    body = response.json()
    assert response.status_code == 200
    assert body["targetHeroId"] == "tigreal"
    assert body["source"] == "fallback"
    assert len(body["recommendations"]) == 2
    assert body["recommendations"][0]["rank"] == 1
    assert body["recommendations"][0]["score"] == 0
    assert body["recommendations"][0]["confidence"] == 0
    assert body["recommendations"][0]["evidenceIds"]


def test_analyze_unknown_target_error_shape() -> None:
    with TestClient(app) as client:
        response = client.post("/api/analyze", json={"targetHeroId": "unknown"})

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "target_hero_not_found",
            "message": "Target hero was not found in the dataset.",
        }
    }
