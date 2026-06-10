from fastapi.testclient import TestClient

from app.core.config import DATA_DIR
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
    # Exercise the no-data branch with controlled test state; the bundled dataset
    # currently has curated synergies for every catalog hero.
    with TestClient(app) as client:
        original_dataset = app.state.dataset
        try:
            app.state.dataset = _SynergyNoDataDataset(Dataset(DATA_DIR), "ling")
            response = client.get("/api/heroes/ling/synergies")
        finally:
            app.state.dataset = original_dataset

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
