"""Shared detail-analysis primitives used by both counter and synergy analyzers.

The "explain one matchup/pairing" response contract is identical for counters and
synergies (score, confidence, summary, strengths, conditions, failureCases, evidenceIds),
so the payload model, evidence validation, and repair flow live here, domain-free.
"""

import json
from typing import Any, Protocol

from pydantic import BaseModel, Field, ValidationError

from app.analyzer.errors import AnalyzerProviderError
from app.analyzer.prompt import language_instruction


class _ProofLike(Protocol):
    id: str


class _MatchupLike(Protocol):
    proof: list[_ProofLike]


class DetailPayload(BaseModel):
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)
    summary: str = Field(min_length=1)
    strengths: list[str] = Field(min_length=1)
    conditions: list[str] = Field(default_factory=list)
    failureCases: list[str] = Field(default_factory=list)
    evidenceIds: list[str] = Field(default_factory=list)


DETAIL_REPAIR_INSTRUCTION = """The previous JSON did not match the required detail response schema.

Return corrected JSON only with exactly these keys:
- score: integer 0-100
- confidence: integer 0-100
- summary: non-empty string
- strengths: non-empty array of strings from the provided reasons/proof
- conditions: array of strings from proof.worksBestWhen
- failureCases: array of strings from proof.failureCases
- evidenceIds: array of proof ids present in the input

Do not add matchup facts outside the provided dataset context."""


def allowed_evidence_ids(matchup: _MatchupLike) -> set[str]:
    return {proof.id for proof in matchup.proof}


def validate_evidence_ids(evidence_ids: list[str], allowed_ids: set[str]) -> bool:
    if not evidence_ids:
        return True
    return all(evidence_id in allowed_ids for evidence_id in evidence_ids)


def build_detail_repair_messages(
    messages: list[dict[str, str]],
    payload: dict[str, Any],
    exc: ValidationError,
    language: str,
) -> list[dict[str, str]]:
    return [
        *messages,
        {"role": "assistant", "content": json.dumps(payload, ensure_ascii=False)},
        {
            "role": "user",
            "content": (
                f"{DETAIL_REPAIR_INSTRUCTION}\n\n"
                f"{language_instruction(language)}\n\n"
                f"Validation error:\n{exc}"
            ),
        },
    ]


def validate_detail_payload(
    payload: dict[str, Any],
    allowed_ids: set[str],
) -> DetailPayload:
    parsed = DetailPayload.model_validate(payload)
    if not validate_evidence_ids(parsed.evidenceIds, allowed_ids):
        raise AnalyzerProviderError("Detail response referenced unknown evidence ids.")
    return parsed
