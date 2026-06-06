import json
from pathlib import Path
from typing import Any

from app.schemas.counter import CounterIndex, CounterMatchup
from app.schemas.hero import Hero
from app.schemas.synergy import SynergyIndex, SynergyMatchup


def load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as file:
        return json.load(file)


def load_heroes(data_dir: Path) -> list[Hero]:
    raw = load_json(data_dir / "heroes.json")
    if not isinstance(raw, list):
        raise ValueError("heroes.json must contain an array")
    return [Hero.model_validate(item) for item in raw]


def load_counter_index(data_dir: Path) -> CounterIndex:
    return CounterIndex.model_validate(load_json(data_dir / "counters.json"))


def load_counter_matchups(data_dir: Path) -> list[CounterMatchup]:
    index = load_counter_index(data_dir)
    matchups: list[CounterMatchup] = []

    for relative_file in index.files:
        path = data_dir / relative_file
        raw = load_json(path)
        if not isinstance(raw, list):
            raise ValueError(f"{relative_file} must contain an array")
        matchups.extend(CounterMatchup.model_validate(item) for item in raw)

    return matchups


def load_counter_matchups_by_file(data_dir: Path) -> dict[str, list[CounterMatchup]]:
    index = load_counter_index(data_dir)
    matchups_by_file: dict[str, list[CounterMatchup]] = {}

    for relative_file in index.files:
        path = data_dir / relative_file
        raw = load_json(path)
        if not isinstance(raw, list):
            raise ValueError(f"{relative_file} must contain an array")
        matchups_by_file[relative_file] = [CounterMatchup.model_validate(item) for item in raw]

    return matchups_by_file


def load_synergy_index(data_dir: Path) -> SynergyIndex:
    return SynergyIndex.model_validate(load_json(data_dir / "synergies.json"))


def load_synergy_matchups(data_dir: Path) -> list[SynergyMatchup]:
    index = load_synergy_index(data_dir)
    matchups: list[SynergyMatchup] = []

    for relative_file in index.files:
        path = data_dir / relative_file
        raw = load_json(path)
        if not isinstance(raw, list):
            raise ValueError(f"{relative_file} must contain an array")
        matchups.extend(SynergyMatchup.model_validate(item) for item in raw)

    return matchups


def load_synergy_matchups_by_file(data_dir: Path) -> dict[str, list[SynergyMatchup]]:
    index = load_synergy_index(data_dir)
    matchups_by_file: dict[str, list[SynergyMatchup]] = {}

    for relative_file in index.files:
        path = data_dir / relative_file
        raw = load_json(path)
        if not isinstance(raw, list):
            raise ValueError(f"{relative_file} must contain an array")
        matchups_by_file[relative_file] = [SynergyMatchup.model_validate(item) for item in raw]

    return matchups_by_file


class Dataset:
    def __init__(self, data_dir: Path) -> None:
        self.data_dir = data_dir
        self.heroes = load_heroes(data_dir)
        self.matchups = load_counter_matchups(data_dir)
        self.synergies = load_synergy_matchups(data_dir)
        self.heroes_by_id = {hero.uid: hero for hero in self.heroes}

    @property
    def ready_targets(self) -> list[str]:
        seen: set[str] = set()
        targets: list[str] = []
        for matchup in self.matchups:
            if matchup.targetHeroId not in seen:
                seen.add(matchup.targetHeroId)
                targets.append(matchup.targetHeroId)
        return targets

    def get_matchups_for_target(self, target_hero_id: str) -> list[CounterMatchup]:
        return [matchup for matchup in self.matchups if matchup.targetHeroId == target_hero_id]

    def get_synergies_for_anchor(self, anchor_hero_id: str) -> list[SynergyMatchup]:
        return [
            synergy for synergy in self.synergies if synergy.anchorHeroId == anchor_hero_id
        ]
