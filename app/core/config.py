from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

DATA_DIR = Path("public/data")
LOCAL_DEV_ORIGIN = "http://localhost:3000"


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "mlbb-analyzer-service"
    host: str = Field(default="127.0.0.1", validation_alias="HOST")
    port: int = Field(default=8000, ge=1, le=65535, validation_alias="PORT")
    reload: bool = Field(default=False, validation_alias="RELOAD")
    frontend_origin: str | None = Field(default=None, validation_alias="FRONTEND_ORIGIN")
    ai_provider: str = Field(default="openai", validation_alias="AI_PROVIDER")
    openai_api_key: str | None = Field(default=None, validation_alias="OPENAI_API_KEY")
    ai_model: str | None = Field(default=None, validation_alias="AI_MODEL")
    ai_timeout_seconds: int = Field(default=20, validation_alias="AI_TIMEOUT_SECONDS")

    def allowed_origins(self) -> list[str]:
        origins = [LOCAL_DEV_ORIGIN]
        if not self.frontend_origin:
            return origins
        for origin in self.frontend_origin.split(","):
            trimmed = origin.strip()
            if trimmed and trimmed not in origins:
                origins.append(trimmed)
        return origins


@lru_cache
def get_settings() -> Settings:
    return Settings()
