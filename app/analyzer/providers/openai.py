"""OpenAI direct provider — implement when AI_PROVIDER=openai is required."""

from typing import Any

from app.analyzer.providers.base import ProviderError
from app.core.config import Settings

PROVIDER_NAME = "OpenAI"


class OpenAIChatProvider:
    """Placeholder for a future OpenAI SDK integration."""

    provider_name = PROVIDER_NAME

    def __init__(self, settings: Settings) -> None:
        if not settings.openai_configured():
            raise ProviderError(f"{PROVIDER_NAME} API key is not configured.")
        raise ProviderError(f"{PROVIDER_NAME} provider is not implemented yet.")

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, Any]:
        raise ProviderError(f"{PROVIDER_NAME} provider is not implemented yet.")

    def close(self) -> None:
        return None
