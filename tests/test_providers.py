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
