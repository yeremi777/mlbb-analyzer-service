import json
from typing import Any

from app.schemas.counter import CounterMatchup, Proof
from app.schemas.hero import Hero
from app.schemas.synergy import SynergyMatchup

SCORING_SYSTEM_INSTRUCTION = """You are scoring Mobile Legends hero counter recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each counter in the batch, return:
- counterHeroId: must match an id from the input
- score: 0-100 matchup strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: hard counter or very direct mechanic counter
- 85-94: strong and reliable counter
- 75-84: good counter with meaningful conditions
- 65-74: situational counter
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill interactions and direct crowd-control counters should score higher than generic damage or item-dependent counters.

Respond with JSON only in this shape:
{"recommendations":[{"counterHeroId":"...","score":0,"confidence":0}]}
Include every counterHeroId from the input exactly once."""

DETAIL_SYSTEM_INSTRUCTION = """You are explaining one Mobile Legends hero counter matchup.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input."""

LANGUAGE_INSTRUCTIONS = {
    "en": (
        "Write all user-visible explanation text (summary, strengths, conditions, failureCases) "
        "in English. Keep every identifier exactly as given: counterHeroId, evidenceIds, and proof ids."
    ),
    "id": (
        "Write all user-visible explanation text (summary, strengths, conditions, failureCases) "
        "in natural Indonesian (Bahasa Indonesia). "
        "Do not translate or alter any identifier: keep counterHeroId, evidenceIds, and proof ids "
        "exactly as given. Keep hero names as written. Only the explanatory prose should be Indonesian."
    ),
}


def language_instruction(language: str) -> str:
    return LANGUAGE_INSTRUCTIONS.get(language, LANGUAGE_INSTRUCTIONS["en"])


def _hero_context(hero: Hero) -> dict[str, Any]:
    return {
        "uid": hero.uid,
        "name": hero.name,
        "roles": hero.roles,
        "lanes": hero.lanes,
    }


def _proof_context(proof: Proof, *, include_detail: bool = True) -> dict[str, Any]:
    context: dict[str, Any] = {
        "id": proof.id,
        "category": proof.category,
        "priority": proof.priority,
        "impact": proof.impact,
        "summary": proof.summary,
    }
    if include_detail:
        context["worksBestWhen"] = proof.worksBestWhen
        context["failureCases"] = proof.failureCases
    return context


def _matchup_context(
    matchup: CounterMatchup,
    counter_hero: Hero | None,
    *,
    include_detail: bool = True,
) -> dict[str, Any]:
    context: dict[str, Any] = {
        "counterHeroId": matchup.counterHeroId,
        "reasons": matchup.reasons,
        "counterTypes": matchup.counterTypes,
        "proof": [
            _proof_context(proof, include_detail=include_detail)
            for proof in matchup.proof
        ],
    }
    if counter_hero is not None:
        context["counterHero"] = _hero_context(counter_hero)
    return context


def _json_context(payload: dict[str, Any]) -> str:
    return json.dumps(payload, ensure_ascii=False, separators=(",", ":"))


def build_scoring_messages(
    target_hero: Hero,
    matchups: list[CounterMatchup],
    heroes_by_id: dict[str, Hero],
    language: str,
) -> list[dict[str, str]]:
    payload = {
        "targetHero": _hero_context(target_hero),
        "matchups": [
            _matchup_context(
                matchup,
                heroes_by_id.get(matchup.counterHeroId),
                include_detail=False,
            )
            for matchup in matchups
        ],
        "outputLanguage": language,
    }
    return [
        {"role": "system", "content": SCORING_SYSTEM_INSTRUCTION},
        {
            "role": "user",
            "content": (
                f"{language_instruction(language)}\n\n"
                f"Dataset context:\n{_json_context(payload)}"
            ),
        },
    ]


def build_detail_messages(
    target_hero: Hero,
    matchup: CounterMatchup,
    counter_hero: Hero,
    language: str,
) -> list[dict[str, str]]:
    payload = {
        "targetHero": _hero_context(target_hero),
        "matchup": _matchup_context(matchup, counter_hero),
        "outputLanguage": language,
    }
    return [
        {"role": "system", "content": DETAIL_SYSTEM_INSTRUCTION},
        {
            "role": "user",
            "content": (
                f"{language_instruction(language)}\n\n"
                f"Dataset context:\n{_json_context(payload)}"
            ),
        },
    ]


SYNERGY_SCORING_SYSTEM_INSTRUCTION = """You are scoring Mobile Legends hero synergy recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each synergy in the batch, return:
- synergyHeroId: must match an id from the input
- score: 0-100 synergy strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: defining, near-mandatory pairing
- 85-94: strong and reliable synergy
- 75-84: good synergy with meaningful conditions
- 65-74: situational synergy
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill-combo and setup-into-payoff synergies should score higher than generic or purely situational pairings.

Respond with JSON only in this shape:
{"recommendations":[{"synergyHeroId":"...","score":0,"confidence":0}]}
Include every synergyHeroId from the input exactly once."""

SYNERGY_DETAIL_SYSTEM_INSTRUCTION = """You are explaining one Mobile Legends hero synergy pairing.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input."""


def _synergy_context(
    synergy: SynergyMatchup,
    synergy_hero: Hero | None,
    *,
    include_detail: bool = True,
) -> dict[str, Any]:
    context: dict[str, Any] = {
        "synergyHeroId": synergy.synergyHeroId,
        "reasons": synergy.reasons,
        "synergyTypes": synergy.synergyTypes,
        "proof": [
            _proof_context(proof, include_detail=include_detail)
            for proof in synergy.proof
        ],
    }
    if synergy_hero is not None:
        context["synergyHero"] = _hero_context(synergy_hero)
    return context


def build_synergy_scoring_messages(
    anchor_hero: Hero,
    synergies: list[SynergyMatchup],
    heroes_by_id: dict[str, Hero],
    language: str,
) -> list[dict[str, str]]:
    payload = {
        "anchorHero": _hero_context(anchor_hero),
        "synergies": [
            _synergy_context(
                synergy,
                heroes_by_id.get(synergy.synergyHeroId),
                include_detail=False,
            )
            for synergy in synergies
        ],
        "outputLanguage": language,
    }
    return [
        {"role": "system", "content": SYNERGY_SCORING_SYSTEM_INSTRUCTION},
        {
            "role": "user",
            "content": (
                f"{language_instruction(language)}\n\n"
                f"Dataset context:\n{_json_context(payload)}"
            ),
        },
    ]


def build_synergy_detail_messages(
    anchor_hero: Hero,
    synergy: SynergyMatchup,
    synergy_hero: Hero,
    language: str,
) -> list[dict[str, str]]:
    payload = {
        "anchorHero": _hero_context(anchor_hero),
        "synergy": _synergy_context(synergy, synergy_hero),
        "outputLanguage": language,
    }
    return [
        {"role": "system", "content": SYNERGY_DETAIL_SYSTEM_INSTRUCTION},
        {
            "role": "user",
            "content": (
                f"{language_instruction(language)}\n\n"
                f"Dataset context:\n{_json_context(payload)}"
            ),
        },
    ]
