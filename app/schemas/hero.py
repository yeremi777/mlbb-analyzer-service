from pydantic import BaseModel, ConfigDict, Field


class HeroImages(BaseModel):
    model_config = ConfigDict(extra="allow")

    head: str | None = None


class Hero(BaseModel):
    model_config = ConfigDict(extra="forbid")

    uid: str = Field(min_length=1)
    mlid: str = Field(min_length=1)
    name: str = Field(min_length=1)
    roles: list[str] = Field(min_length=1)
    lanes: list[str] = Field(default_factory=list)
    images: HeroImages = Field(default_factory=HeroImages)


class HeroListResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "items": [
                    {
                        "uid": "tigreal",
                        "mlid": "6",
                        "name": "Tigreal",
                        "roles": ["tank"],
                        "lanes": ["roam"],
                        "images": {"head": "https://example.com/tigreal.png"},
                    }
                ],
                "page": 1,
                "size": 10,
                "total": 1,
                "pages": 1,
            }
        }
    )

    items: list[Hero]
    page: int = Field(ge=1)
    size: int = Field(ge=1)
    total: int = Field(ge=0)
    pages: int = Field(ge=0)
