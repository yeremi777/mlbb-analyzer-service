from functools import lru_cache
from pathlib import Path
from typing import Literal

from pydantic import AliasChoices, Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

DATA_DIR = Path(__file__).resolve().parents[1] / "data" / "static"
LOCAL_DEV_ORIGIN = "http://localhost:3000"
PLACEHOLDER_API_KEYS = frozenset(
    {
        "",
        "<YOUR_OPENROUTER_API_KEY>",
        "<YOUR_OPENCODE_ZEN_API_KEY>",
    }
)


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "mlbb-analyzer-service"
    host: str = Field(default="127.0.0.1", validation_alias="HOST")
    port: int = Field(default=8000, ge=1, le=65535, validation_alias="PORT")
    reload: bool = Field(default=False, validation_alias="RELOAD")
    frontend_origin: str | None = Field(default=None, validation_alias="FRONTEND_ORIGIN")

    ai_provider: str = Field(
        default="openrouter",
        validation_alias=AliasChoices("AI_PROVIDERS", "AI_PROVIDER"),
    )
    ai_timeout_seconds: int = Field(default=20, validation_alias="AI_TIMEOUT_SECONDS")
    ai_analysis_cache_ttl_seconds: int = Field(
        default=600,
        ge=0,
        validation_alias="AI_ANALYSIS_CACHE_TTL_SECONDS",
    )
    ai_analysis_cache_max_entries: int = Field(
        default=256,
        ge=0,
        validation_alias="AI_ANALYSIS_CACHE_MAX_ENTRIES",
    )

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

    opencode_zen_api_key: str | None = Field(default=None, validation_alias="OPENCODE_ZEN_API_KEY")
    opencode_zen_model: str | None = Field(default=None, validation_alias="OPENCODE_ZEN_MODEL")
    opencode_zen_server_url: str = Field(
        default="https://opencode.ai/zen/v1",
        validation_alias="OPENCODE_ZEN_SERVER_URL",
    )
    opencode_zen_endpoint_type: Literal["chat_completions", "responses", "messages"] = Field(
        default="chat_completions",
        validation_alias="OPENCODE_ZEN_ENDPOINT_TYPE",
    )

    redis_url_override: str | None = Field(default=None, validation_alias="REDIS_URL")
    redis_host: str | None = Field(default=None, validation_alias="REDIS_HOST")
    redis_port: int = Field(default=6379, ge=1, le=65535, validation_alias="REDIS_PORT")
    redis_db: int = Field(default=0, ge=0, validation_alias="REDIS_DB")
    rate_limit_enabled: bool = Field(default=False, validation_alias="RATE_LIMIT_ENABLED")
    rate_limit_analyze_max_requests: int = Field(
        default=5,
        ge=1,
        validation_alias="RATE_LIMIT_ANALYZE_MAX_REQUESTS",
    )
    rate_limit_analyze_window_seconds: int = Field(
        default=18_000,
        ge=1,
        validation_alias="RATE_LIMIT_ANALYZE_WINDOW_SECONDS",
    )
    rate_limit_analyze_detail_multiplier: int = Field(
        default=3,
        ge=1,
        validation_alias="RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER",
    )
    rate_limit_cookie_name: str = Field(
        default="mlbb_analyzer_client_id",
        validation_alias="RATE_LIMIT_COOKIE_NAME",
    )
    rate_limit_cookie_max_age_seconds: int = Field(
        default=2_592_000,
        ge=1,
        validation_alias="RATE_LIMIT_COOKIE_MAX_AGE_SECONDS",
    )
    rate_limit_cookie_secure: bool = Field(
        default=False,
        validation_alias="RATE_LIMIT_COOKIE_SECURE",
    )
    rate_limit_cookie_samesite: Literal["lax", "strict", "none"] = Field(
        default="lax",
        validation_alias="RATE_LIMIT_COOKIE_SAMESITE",
    )
    rate_limit_salt: str = Field(
        default="mlbb-analyzer-service-local-rate-limit",
        validation_alias="RATE_LIMIT_SALT",
    )

    @field_validator("rate_limit_cookie_samesite", mode="before")
    @classmethod
    def normalize_cookie_samesite(cls, value: str) -> str:
        return value.strip().casefold()

    def allowed_origins(self) -> list[str]:
        origins = [LOCAL_DEV_ORIGIN]
        if not self.frontend_origin:
            return origins
        for origin in self.frontend_origin.split(","):
            trimmed = origin.strip()
            if trimmed and trimmed not in origins:
                origins.append(trimmed)
        return origins

    def redis_url(self) -> str | None:
        if self.redis_url_override:
            return self.redis_url_override
        if not self.redis_host:
            return None
        return f"redis://{self.redis_host}:{self.redis_port}/{self.redis_db}"

    def _api_key_configured(self, key: str | None) -> bool:
        return bool(key and key not in PLACEHOLDER_API_KEYS)

    def openrouter_configured(self) -> bool:
        return self._api_key_configured(self.openrouter_api_key)

    def openai_configured(self) -> bool:
        return self._api_key_configured(self.openai_api_key)

    def opencode_zen_configured(self) -> bool:
        return self._api_key_configured(self.opencode_zen_api_key)

    def ai_provider_slugs(self) -> list[str]:
        return [
            provider.strip().casefold()
            for provider in self.ai_provider.split(",")
            if provider.strip()
        ]

    def ai_enabled(self) -> bool:
        return any(self.provider_configured(provider) for provider in self.ai_provider_slugs())

    def provider_configured(self, provider: str) -> bool:
        if provider == "openrouter":
            return self.openrouter_configured()
        if provider == "opencode_zen":
            return self.opencode_zen_configured()
        if provider == "openai":
            return self.openai_configured()
        return False

    def active_model(self) -> str | None:
        providers = self.ai_provider_slugs()
        if not providers:
            return None
        return self.provider_model(providers[0])

    def provider_model(self, provider: str) -> str | None:
        if provider == "openrouter":
            return self.openrouter_model
        if provider == "opencode_zen":
            return self.opencode_zen_model
        if provider == "openai":
            return self.openai_model
        return None


@lru_cache
def get_settings() -> Settings:
    return Settings()
