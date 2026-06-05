import pytest

from app.analyzer.providers import ProviderError, create_chat_provider
from app.core.config import Settings


def test_create_chat_provider_returns_openrouter() -> None:
    settings = Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
    )
    provider = create_chat_provider(settings)
    assert provider.provider_name == "OpenRouter"
    provider.close()


def test_create_chat_provider_rejects_unknown_provider() -> None:
    settings = Settings(AI_PROVIDER="anthropic", OPENROUTER_API_KEY="test-key")
    with pytest.raises(ProviderError, match="not implemented"):
        create_chat_provider(settings)


def test_create_chat_provider_rejects_openai_as_not_implemented() -> None:
    settings = Settings(AI_PROVIDER="openai", OPENAI_API_KEY="test-key")
    with pytest.raises(ProviderError, match="not implemented"):
        create_chat_provider(settings)


def test_settings_builds_redis_url_from_host_port_and_db() -> None:
    settings = Settings(REDIS_HOST="127.0.0.1", REDIS_PORT=6379, REDIS_DB=0)

    assert settings.redis_url() == "redis://127.0.0.1:6379/0"


def test_settings_prefers_redis_url_override() -> None:
    settings = Settings(
        REDIS_URL="rediss://managed-redis.example.com:6380/0",
        REDIS_HOST="127.0.0.1",
        REDIS_PORT=6379,
        REDIS_DB=0,
    )

    assert settings.redis_url() == "rediss://managed-redis.example.com:6380/0"


def test_settings_normalizes_cookie_samesite() -> None:
    settings = Settings(RATE_LIMIT_COOKIE_SAMESITE="None")

    assert settings.rate_limit_cookie_samesite == "none"
