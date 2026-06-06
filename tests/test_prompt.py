from app.analyzer.prompt import (
    DETAIL_SYSTEM_INSTRUCTION,
    SCORING_SYSTEM_INSTRUCTION,
    build_detail_messages,
    build_scoring_messages,
)
from app.core.config import DATA_DIR
from app.data.loader import Dataset


def test_scoring_prompt_includes_guardrails_and_tigreal_context() -> None:
    dataset = Dataset(DATA_DIR)
    target = dataset.heroes_by_id["tigreal"]
    matchups = dataset.get_matchups_for_target("tigreal")

    messages = build_scoring_messages(target, matchups, dataset.heroes_by_id, "en")
    combined = "\n".join(message["content"] for message in messages)

    assert messages[0]["content"] == SCORING_SYSTEM_INSTRUCTION
    assert "Do not invent" in combined
    assert "tigreal" in combined
    assert "diggie" in combined
    assert '"proof"' in combined
    assert "counterHeroId" in combined


def test_detail_prompt_includes_single_matchup_context() -> None:
    dataset = Dataset(DATA_DIR)
    target = dataset.heroes_by_id["tigreal"]
    matchups = dataset.get_matchups_for_target("tigreal")
    matchup = matchups[0]
    counter = dataset.heroes_by_id[matchup.counterHeroId]

    messages = build_detail_messages(target, matchup, counter, "en")
    combined = "\n".join(message["content"] for message in messages)

    assert messages[0]["content"] == DETAIL_SYSTEM_INSTRUCTION
    assert matchup.counterHeroId in combined
    assert "failureCases" in combined
    assert "evidenceIds" in combined


def test_detail_prompt_requests_indonesian_output_and_preserves_ids() -> None:
    dataset = Dataset(DATA_DIR)
    target = dataset.heroes_by_id["tigreal"]
    matchups = dataset.get_matchups_for_target("tigreal")
    matchup = matchups[0]
    counter = dataset.heroes_by_id[matchup.counterHeroId]

    messages = build_detail_messages(target, matchup, counter, "id")
    user_content = messages[1]["content"]

    assert "Indonesian" in user_content
    assert "Do not translate or alter any identifier" in user_content
    # identifiers must still be present unchanged in the dataset context
    assert matchup.counterHeroId in user_content


def test_unknown_language_falls_back_to_english() -> None:
    dataset = Dataset(DATA_DIR)
    target = dataset.heroes_by_id["tigreal"]
    matchups = dataset.get_matchups_for_target("tigreal")
    matchup = matchups[0]
    counter = dataset.heroes_by_id[matchup.counterHeroId]

    messages = build_detail_messages(target, matchup, counter, "fr")

    assert "in English" in messages[1]["content"]
