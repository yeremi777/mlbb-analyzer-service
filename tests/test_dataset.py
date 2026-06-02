from app.core.config import DATA_DIR
from app.data.loader import Dataset
from app.data.validation import validate_dataset


def test_dataset_validates() -> None:
    validate_dataset(DATA_DIR)


def test_dataset_loads_ready_targets() -> None:
    dataset = Dataset(DATA_DIR)

    assert dataset.ready_targets == [
        "alice",
        "alucard",
        "balmond",
        "miya",
        "nana",
        "saber",
        "tigreal",
    ]
    assert len(dataset.matchups) > 0
