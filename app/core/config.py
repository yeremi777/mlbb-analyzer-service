from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "mlbb-analyzer-service"
    data_dir: Path = Field(default=Path("data"), validation_alias="DATA_DIR")
    frontend_origin: str = Field(
        default="http://localhost:3000",
        validation_alias="FRONTEND_ORIGIN",
    )
    ai_provider: str = Field(default="openai", validation_alias="AI_PROVIDER")
    openai_api_key: str | None = Field(default=None, validation_alias="OPENAI_API_KEY")
    ai_model: str | None = Field(default=None, validation_alias="AI_MODEL")
    ai_timeout_seconds: int = Field(default=20, validation_alias="AI_TIMEOUT_SECONDS")


@lru_cache
def get_settings() -> Settings:
    return Settings()

