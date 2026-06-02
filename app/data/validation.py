from pathlib import Path

from pydantic import ValidationError

from app.data.loader import (
    load_counter_index,
    load_counter_matchups,
    load_counter_matchups_by_file,
    load_heroes,
    load_json,
)


class DatasetValidationError(ValueError):
    def __init__(self, errors: list[str]) -> None:
        self.errors = errors
        super().__init__("\n".join(errors))


def _has_blank(values: list[str]) -> bool:
    return any(not value.strip() for value in values)


def validate_dataset(data_dir: Path) -> None:
    errors: list[str] = []

    try:
        heroes_raw = load_json(data_dir / "heroes.json")
        if not isinstance(heroes_raw, list):
            errors.append("heroes.json must contain an array")
            heroes = []
        else:
            heroes = load_heroes(data_dir)
    except (OSError, ValueError, ValidationError) as exc:
        raise DatasetValidationError([f"heroes.json is invalid: {exc}"]) from exc

    hero_ids = {hero.uid for hero in heroes}

    try:
        index = load_counter_index(data_dir)
    except (OSError, ValueError, ValidationError) as exc:
        raise DatasetValidationError([f"counters.json is invalid: {exc}"]) from exc

    for relative_file in index.files:
        path = data_dir / relative_file
        if path.resolve().parent != (data_dir / "counters").resolve():
            errors.append(f"{relative_file} must be inside data/counters")
        if path.suffix != ".json":
            errors.append(f"{relative_file} must be a JSON file")

    try:
        matchups = load_counter_matchups(data_dir)
        matchups_by_file = load_counter_matchups_by_file(data_dir)
    except (OSError, ValueError, ValidationError) as exc:
        raise DatasetValidationError([*errors, f"counter files are invalid: {exc}"]) from exc

    seen_pairs: set[tuple[str, str]] = set()

    for relative_file, file_matchups in matchups_by_file.items():
        expected_target = Path(relative_file).stem
        for matchup in file_matchups:
            if matchup.targetHeroId != expected_target:
                errors.append(
                    f"{relative_file} contains targetHeroId {matchup.targetHeroId}, "
                    f"expected {expected_target}"
                )

    for matchup in matchups:
        pair = (matchup.targetHeroId, matchup.counterHeroId)

        if matchup.targetHeroId not in hero_ids:
            errors.append(f"{matchup.targetHeroId} targetHeroId does not exist in heroes.json")
        if matchup.counterHeroId not in hero_ids:
            errors.append(f"{matchup.counterHeroId} counterHeroId does not exist in heroes.json")
        if matchup.targetHeroId == matchup.counterHeroId:
            errors.append(f"{matchup.targetHeroId} cannot counter itself")
        if pair in seen_pairs:
            errors.append(f"duplicate matchup: {matchup.targetHeroId}/{matchup.counterHeroId}")
        seen_pairs.add(pair)
        if _has_blank(matchup.reasons):
            errors.append(f"{matchup.targetHeroId}/{matchup.counterHeroId} has blank reasons")
        if _has_blank(matchup.counterTypes):
            errors.append(f"{matchup.targetHeroId}/{matchup.counterHeroId} has blank counterTypes")
        if "score" in matchup.model_extra:
            errors.append(f"{matchup.targetHeroId}/{matchup.counterHeroId} must not include score")
        for proof in matchup.proof:
            if "scoreHint" in proof.model_extra:
                errors.append(f"{proof.id} must not include scoreHint")

    if errors:
        raise DatasetValidationError(errors)
