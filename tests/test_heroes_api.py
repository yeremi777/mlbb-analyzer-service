from fastapi.testclient import TestClient

from app.main import app


def test_list_heroes_defaults_to_first_page() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes")

    body = response.json()
    assert response.status_code == 200
    assert body["page"] == 1
    assert body["size"] == 10
    assert body["total"] > 10
    assert body["items"][0]["uid"] == "miya"


def test_list_heroes_filters_by_search_role_and_lane() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes", params={"search": "tig", "role": "tank", "lane": "roam"})

    body = response.json()
    assert response.status_code == 200
    assert body["total"] == 1
    assert body["items"][0]["uid"] == "tigreal"


def test_list_heroes_paginates() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes", params={"page": 2, "size": 5})

    body = response.json()
    assert response.status_code == 200
    assert body["page"] == 2
    assert body["size"] == 5
    assert len(body["items"]) == 5


def test_get_hero_detail() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes/tigreal")

    body = response.json()
    assert response.status_code == 200
    assert body["uid"] == "tigreal"
    assert body["name"] == "Tigreal"


def test_get_hero_detail_not_found() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes/unknown")

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "hero_not_found",
            "message": "Hero was not found in the dataset.",
        }
    }


def test_list_hero_counters() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes/tigreal/counters")

    body = response.json()
    assert response.status_code == 200
    assert len(body) > 0
    assert body[0]["targetHeroId"] == "tigreal"
    assert body[0]["counterHero"]["uid"] == "diggie"
    assert body[0]["reasons"]
    assert body[0]["proof"]


def test_list_hero_synergies() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes/miya/synergies")

    body = response.json()
    assert response.status_code == 200
    assert len(body) == 5
    assert body[0]["anchorHeroId"] == "miya"
    assert body[0]["synergyHero"]["uid"]
    assert body[0]["synergyTypes"]
    assert body[0]["reasons"]
    assert body[0]["proof"]


def test_list_hero_synergies_no_data() -> None:
    # layla exists in the catalog but is not one of the curated synergy anchors.
    with TestClient(app) as client:
        response = client.get("/api/heroes/layla/synergies")

    assert response.status_code == 404
    assert response.json() == {
        "error": {
            "code": "synergy_data_not_found",
            "message": "Synergy data was not found for the anchor hero.",
        }
    }


def test_list_hero_synergies_unknown_hero() -> None:
    with TestClient(app) as client:
        response = client.get("/api/heroes/unknown/synergies")

    assert response.status_code == 404
    assert response.json()["error"]["code"] == "hero_not_found"


def test_dataset_summary_route_removed() -> None:
    with TestClient(app) as client:
        response = client.get("/api/dataset/summary")

    assert response.status_code == 404
