from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class AnalyzeRequest(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroId": "tigreal",
                "limit": 5,
                "language": "en",
            }
        }
    )

    targetHeroId: str = Field(
        min_length=1,
        description="Target enemy hero UID from the local dataset.",
        examples=["tigreal"],
    )
    limit: int = Field(
        default=5,
        ge=1,
        le=20,
        description="Maximum number of counter recommendations to return.",
    )
    language: Literal["en", "id"] = Field(
        default="en",
        description="Preferred output language. Indonesian support is reserved for a later milestone.",
    )


class Recommendation(BaseModel):
    rank: int = Field(ge=1, description="Recommendation rank after analyzer ordering.")
    counterHeroId: str = Field(description="Recommended counter hero UID.")
    score: int = Field(ge=0, le=100, description="Runtime matchup strength score.")
    confidence: int = Field(ge=0, le=100, description="Runtime evidence confidence score.")
    summary: str = Field(description="Concise recommendation explanation.")
    strengths: list[str] = Field(description="Concrete strengths from provided dataset evidence.")
    conditions: list[str] = Field(description="Conditions required for the matchup to work well.")
    failureCases: list[str] = Field(description="Known ways the matchup can fail.")
    evidenceIds: list[str] = Field(description="Proof IDs used for the recommendation.")


class AnalyzeResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroId": "tigreal",
                "source": "fallback",
                "recommendations": [
                    {
                        "rank": 1,
                        "counterHeroId": "diggie",
                        "score": 0,
                        "confidence": 0,
                        "summary": "Diggie reduces the impact of Tigreal's crowd-control engage.",
                        "strengths": [
                            "Diggie reduces the impact of Tigreal's crowd-control engage."
                        ],
                        "conditions": [
                            "Diggie saves ultimate for Tigreal's real engage."
                        ],
                        "failureCases": [
                            "Tigreal baits Diggie's ultimate before committing."
                        ],
                        "evidenceIds": ["diggie-time-journey-vs-tigreal-engage"],
                    }
                ],
            }
        }
    )

    targetHeroId: str
    source: Literal["ai", "fallback"]
    recommendations: list[Recommendation]


class ErrorDetail(BaseModel):
    code: str
    message: str


class ErrorResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "error": {
                    "code": "target_hero_not_found",
                    "message": "Target hero was not found in the dataset.",
                }
            }
        }
    )

    error: ErrorDetail


class DatasetSummary(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroCount": 7,
                "counterMatchupCount": 33,
                "readyTargets": [
                    "alice",
                    "alucard",
                    "balmond",
                    "miya",
                    "nana",
                    "saber",
                    "tigreal",
                ],
            }
        }
    )

    targetHeroCount: int
    counterMatchupCount: int
    readyTargets: list[str]
