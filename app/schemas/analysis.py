from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class AnalyzeScoresRequest(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroId": "tigreal",
                "language": "en",
            }
        }
    )

    targetHeroId: str = Field(min_length=1, examples=["tigreal"])
    language: Literal["en", "id"] = Field(default="en")


class ScoreRecommendation(BaseModel):
    rank: int = Field(ge=1)
    counterHeroId: str
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)


class AnalyzeScoresResponse(BaseModel):
    targetHeroId: str
    source: Literal["ai"] = "ai"
    recommendations: list[ScoreRecommendation]


class AnalyzeDetailRequest(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "targetHeroId": "tigreal",
                "counterHeroId": "diggie",
                "language": "en",
            }
        }
    )

    targetHeroId: str = Field(min_length=1, examples=["tigreal"])
    counterHeroId: str = Field(min_length=1, examples=["diggie"])
    language: Literal["en", "id"] = Field(default="en")


class AnalyzeDetailResponse(BaseModel):
    targetHeroId: str
    counterHeroId: str
    source: Literal["ai"] = "ai"
    score: int = Field(ge=0, le=100)
    confidence: int = Field(ge=0, le=100)
    summary: str
    strengths: list[str]
    conditions: list[str]
    failureCases: list[str]
    evidenceIds: list[str]


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
