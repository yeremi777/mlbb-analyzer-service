from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

DATA_DIR = Path(__file__).resolve().parents[1] / "data" / "static"
LOCAL_DEV_ORIGIN = "http://localhost:3000"
PLACEHOLDER_API_KEYS = frozenset({"", "<YOUR_OPENROUTER_API_KEY>"})


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "mlbb-analyzer-service"
    host: str = Field(default="127.0.0.1", validation_alias="HOST")
    port: int = Field(default=8000, ge=1, le=65535, validation_alias="PORT")
    reload: bool = Field(default=False, validation_alias="RELOAD")
    frontend_origin: str | None = Field(default=None, validation_alias="FRONTEND_ORIGIN")

    ai_provider: str = Field(default="openrouter", validation_alias="AI_PROVIDER")
    ai_timeout_seconds: int = Field(default=20, validation_alias="AI_TIMEOUT_SECONDS")

    openai_api_key: str | None = Field(default=None, validation_alias="OPENAI_API_KEY")
    openai_model: str | None = Field(default=None, validation_alias="OPENAI_MODEL")

    openrouter_api_key: str | None = Field(default=None, validation_alias="OPENROUTER_API_KEY")
    openrouter_model: str = Field(default="openrouter/free", validation_alias="OPENROUTER_MODEL")
    openrouter_server_url: str = Field(
        default="https://openrouter.ai/api/v1",
        validation_alias="OPENROUTER_SERVER_URL",
    )
    openrouter_app_title: str = Field(
        default="MLBB Analyzer Service",
        validation_alias="OPENROUTER_APP_TITLE",
    )
    openrouter_http_referer: str = Field(
        default="http://127.0.0.1:8000",
        validation_alias="OPENROUTER_HTTP_REFERER",
    )

    def allowed_origins(self) -> list[str]:
        origins = [LOCAL_DEV_ORIGIN]
        if not self.frontend_origin:
            return origins
        for origin in self.frontend_origin.split(","):
            trimmed = origin.strip()
            if trimmed and trimmed not in origins:
                origins.append(trimmed)
        return origins

    def _api_key_configured(self, key: str | None) -> bool:
        return bool(key and key not in PLACEHOLDER_API_KEYS)

    def openrouter_configured(self) -> bool:
        return self._api_key_configured(self.openrouter_api_key)

    def openai_configured(self) -> bool:
        return self._api_key_configured(self.openai_api_key)

    def ai_enabled(self) -> bool:
        provider = self.ai_provider.strip().casefold()
        if provider == "openrouter":
            return self.openrouter_configured()
        if provider == "openai":
            return self.openai_configured()
        return False

    def active_model(self) -> str | None:
        provider = self.ai_provider.strip().casefold()
        if provider == "openrouter":
            return self.openrouter_model
        if provider == "openai":
            return self.openai_model
        return None


@lru_cache
def get_settings() -> Settings:
    return Settings()
