import json
import sys
from io import StringIO

from app.core.config import DATA_DIR
from app.data.loader import Dataset, load_counter_index
from app.data.validation import main, validate_dataset


def test_dataset_validates() -> None:
    validate_dataset(DATA_DIR)


def test_dataset_loads_ready_targets() -> None:
    dataset = Dataset(DATA_DIR)
    index = load_counter_index(DATA_DIR)

    assert dataset.ready_targets == [file.removeprefix("counters/").removesuffix(".json") for file in index.files]
    assert len(dataset.matchups) > 0


def test_dataset_validation_command_passes_for_bundled_dataset(monkeypatch) -> None:
    stdout = StringIO()
    stderr = StringIO()
    monkeypatch.setattr(sys, "stdout", stdout)
    monkeypatch.setattr(sys, "stderr", stderr)

    assert main([]) == 0
    assert "Dataset validation passed" in stdout.getvalue()
    assert stderr.getvalue() == ""


def test_dataset_validation_command_fails_for_invalid_dataset(tmp_path, monkeypatch) -> None:
    stdout = StringIO()
    stderr = StringIO()
    monkeypatch.setattr(sys, "stdout", stdout)
    monkeypatch.setattr(sys, "stderr", stderr)

    data_dir = tmp_path / "static"
    data_dir.mkdir()
    (data_dir / "counters").mkdir()
    (data_dir / "heroes.json").write_text("[]", encoding="utf-8")
    (data_dir / "counters.json").write_text(
        json.dumps({"files": ["counters/miya.json"]}),
        encoding="utf-8",
    )
    (data_dir / "counters" / "miya.json").write_text(
        json.dumps(
            [
                {
                    "targetHeroId": "miya",
                    "counterHeroId": "unknown",
                    "reasons": ["Invalid hero IDs should fail validation."],
                    "counterTypes": ["invalid"],
                    "proof": [
                        {
                            "id": "invalid-proof",
                            "category": "skill-interaction",
                            "priority": "primary",
                            "impact": "high",
                            "summary": "Invalid test proof.",
                        }
                    ],
                }
            ]
        ),
        encoding="utf-8",
    )

    assert main(["--data-dir", str(data_dir)]) == 1
    assert stdout.getvalue() == ""
    assert "Dataset validation failed" in stderr.getvalue()
    assert "targetHeroId does not exist" in stderr.getvalue()
