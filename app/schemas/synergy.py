from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

from app.schemas.counter import ProofImpact, ProofPriority

SynergyProofCategory = Literal[
    "skill-interaction",
    "crowd-control-chain",
    "engage-follow-up",
    "setup-combo",
    "damage-amplification",
    "protection",
    "peel",
    "frontline-enabler",
    "mobility-enabler",
    "vision-setup",
    "healing-sustain",
    "shielding",
    "poke-siege",
    "pickoff-combo",
    "teamfight-combo",
    "objective-control",
    "laning-synergy",
    "game-phase",
    "positioning-requirement",
    "cooldown-window",
    "execution-difficulty",
]


class SynergyIndex(BaseModel):
    model_config = ConfigDict(extra="forbid")

    files: list[str] = Field(min_length=1)


class SynergyProof(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str = Field(min_length=1)
    category: SynergyProofCategory
    priority: ProofPriority
    impact: ProofImpact
    summary: str = Field(min_length=1)
    worksBestWhen: list[str] = Field(default_factory=list)
    failureCases: list[str] = Field(default_factory=list)


class SynergyMatchup(BaseModel):
    model_config = ConfigDict(extra="allow")

    anchorHeroId: str = Field(min_length=1)
    synergyHeroId: str = Field(min_length=1)
    reasons: list[str] = Field(min_length=1)
    synergyTypes: list[str] = Field(min_length=1)
    proof: list[SynergyProof] = Field(min_length=1)


class SynergyHeroMatchup(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "anchorHeroId": "tigreal",
                "synergyHero": {
                    "uid": "pharsa",
                    "mlid": "76",
                    "name": "Pharsa",
                    "roles": ["mage"],
                    "lanes": ["mid"],
                    "images": {"head": "https://example.com/pharsa.png"},
                },
                "reasons": ["Tigreal groups enemies so Pharsa can land long-range AoE burst."],
                "synergyTypes": ["engage-follow-up", "teamfight-combo"],
                "proof": [
                    {
                        "id": "pharsa-feathered-air-strike-with-tigreal-grouped-aoe",
                        "category": "teamfight-combo",
                        "priority": "primary",
                        "impact": "high",
                        "summary": "Tigreal locks a cluster; Pharsa rains AoE on the immobilized group.",
                        "worksBestWhen": ["Tigreal lands Implosion on two or more enemies."],
                        "failureCases": ["Enemies are spread out so the AoE misses."],
                    }
                ],
            }
        }
    )

    anchorHeroId: str
    synergyHero: "Hero"
    reasons: list[str]
    synergyTypes: list[str]
    proof: list[SynergyProof]


from app.schemas.hero import Hero  # noqa: E402

SynergyHeroMatchup.model_rebuild()
