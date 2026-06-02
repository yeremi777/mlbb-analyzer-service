from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


ProofCategory = Literal[
    "skill-interaction",
    "crowd-control-counter",
    "damage-type-advantage",
    "item-power-spike",
    "mobility-advantage",
    "range-advantage",
    "kiting",
    "sustain-anti-sustain",
    "positioning-requirement",
    "cooldown-window",
    "vision-awareness",
    "teamfight-role-counter",
    "game-phase",
    "execution-difficulty",
]
ProofPriority = Literal["primary", "secondary", "condition"]
ProofImpact = Literal["high", "medium", "low"]


class CounterIndex(BaseModel):
    model_config = ConfigDict(extra="forbid")

    files: list[str] = Field(min_length=1)


class Proof(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str = Field(min_length=1)
    category: ProofCategory
    priority: ProofPriority
    impact: ProofImpact
    summary: str = Field(min_length=1)
    worksBestWhen: list[str] = Field(default_factory=list)
    failureCases: list[str] = Field(default_factory=list)


class CounterMatchup(BaseModel):
    model_config = ConfigDict(extra="allow")

    targetHeroId: str = Field(min_length=1)
    counterHeroId: str = Field(min_length=1)
    reasons: list[str] = Field(min_length=1)
    counterTypes: list[str] = Field(min_length=1)
    proof: list[Proof] = Field(min_length=1)
    patchVersion: str = Field(min_length=1)


class CounterHeroMatchup(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroId": "tigreal",
                "counterHero": {
                    "uid": "diggie",
                    "mlid": "48",
                    "name": "Diggie",
                    "roles": ["support"],
                    "lanes": ["roam"],
                    "images": {"head": "https://example.com/diggie.png"},
                },
                "reasons": ["Diggie reduces the impact of Tigreal's crowd-control engage."],
                "counterTypes": ["anti-cc", "disengage", "teamfight"],
                "proof": [
                    {
                        "id": "diggie-time-journey-vs-tigreal-engage",
                        "category": "skill-interaction",
                        "priority": "primary",
                        "impact": "high",
                        "summary": "Diggie directly answers Tigreal's engage window.",
                        "worksBestWhen": ["Diggie saves ultimate for Tigreal's real engage."],
                        "failureCases": ["Tigreal baits Diggie's ultimate before committing."],
                    }
                ],
                "patchVersion": "starter-v1",
            }
        }
    )

    targetHeroId: str
    counterHero: "Hero"
    reasons: list[str]
    counterTypes: list[str]
    proof: list[Proof]
    patchVersion: str


from app.schemas.hero import Hero  # noqa: E402

CounterHeroMatchup.model_rebuild()
