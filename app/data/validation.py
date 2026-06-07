import argparse
import sys
from pathlib import Path

from pydantic import ValidationError

from app.core.config import DATA_DIR
from app.data.loader import (
    load_counter_index,
    load_counter_matchups,
    load_counter_matchups_by_file,
    load_heroes,
    load_json,
    load_synergy_index,
    load_synergy_matchups,
    load_synergy_matchups_by_file,
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
            errors.append(f"{relative_file} must be inside {data_dir / 'counters'}")
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

    _validate_synergies(data_dir, hero_ids, errors)

    if errors:
        raise DatasetValidationError(errors)


def _validate_synergies(data_dir: Path, hero_ids: set[str], errors: list[str]) -> None:
    try:
        index = load_synergy_index(data_dir)
    except (OSError, ValueError, ValidationError) as exc:
        raise DatasetValidationError([*errors, f"synergies.json is invalid: {exc}"]) from exc

    for relative_file in index.files:
        path = data_dir / relative_file
        if path.resolve().parent != (data_dir / "synergies").resolve():
            errors.append(f"{relative_file} must be inside {data_dir / 'synergies'}")
        if path.suffix != ".json":
            errors.append(f"{relative_file} must be a JSON file")

    try:
        synergies = load_synergy_matchups(data_dir)
        synergies_by_file = load_synergy_matchups_by_file(data_dir)
    except (OSError, ValueError, ValidationError) as exc:
        raise DatasetValidationError([*errors, f"synergy files are invalid: {exc}"]) from exc

    for relative_file, file_synergies in synergies_by_file.items():
        expected_anchor = Path(relative_file).stem
        for synergy in file_synergies:
            if synergy.anchorHeroId != expected_anchor:
                errors.append(
                    f"{relative_file} contains anchorHeroId {synergy.anchorHeroId}, "
                    f"expected {expected_anchor}"
                )

    seen_pairs: set[tuple[str, str]] = set()

    for synergy in synergies:
        pair = (synergy.anchorHeroId, synergy.synergyHeroId)

        if synergy.anchorHeroId not in hero_ids:
            errors.append(f"{synergy.anchorHeroId} anchorHeroId does not exist in heroes.json")
        if synergy.synergyHeroId not in hero_ids:
            errors.append(f"{synergy.synergyHeroId} synergyHeroId does not exist in heroes.json")
        if synergy.anchorHeroId == synergy.synergyHeroId:
            errors.append(f"{synergy.anchorHeroId} cannot synergize with itself")
        if pair in seen_pairs:
            errors.append(f"duplicate synergy: {synergy.anchorHeroId}/{synergy.synergyHeroId}")
        seen_pairs.add(pair)
        if _has_blank(synergy.reasons):
            errors.append(f"{synergy.anchorHeroId}/{synergy.synergyHeroId} has blank reasons")
        if _has_blank(synergy.synergyTypes):
            errors.append(f"{synergy.anchorHeroId}/{synergy.synergyHeroId} has blank synergyTypes")
        if "score" in synergy.model_extra:
            errors.append(f"{synergy.anchorHeroId}/{synergy.synergyHeroId} must not include score")
        for proof in synergy.proof:
            if "scoreHint" in proof.model_extra:
                errors.append(f"{proof.id} must not include scoreHint")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate the bundled MLBB static dataset.")
    parser.add_argument(
        "--data-dir",
        type=Path,
        default=DATA_DIR,
        help=f"Dataset directory to validate. Defaults to {DATA_DIR}.",
    )
    args = parser.parse_args(argv)

    try:
        validate_dataset(args.data_dir)
    except DatasetValidationError as exc:
        print(f"Dataset validation failed for {args.data_dir}:", file=sys.stderr)
        for error in exc.errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"Dataset validation passed for {args.data_dir}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
