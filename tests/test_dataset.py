from app.core.config import DATA_DIR
from app.data.loader import Dataset
from app.data.validation import validate_dataset


def test_dataset_validates() -> None:
    validate_dataset(DATA_DIR)


def test_dataset_loads_ready_targets() -> None:
    dataset = Dataset(DATA_DIR)

    assert dataset.ready_targets == [
        "akai",
        "alice",
        "alpha",
        "alucard",
        "balmond",
        "bane",
        "bruno",
        "chou",
        "clint",
        "eudora",
        "fanny",
        "franco",
        "freya",
        "gord",
        "hayabusa",
        "kagura",
        "karina",
        "layla",
        "lolita",
        "minotaur",
        "miya",
        "nana",
        "natalia",
        "rafaela",
        "ruby",
        "saber",
        "sun",
        "tigreal",
        "yi-sun-shin",
        "zilong",
    ]
    assert len(dataset.matchups) > 0
