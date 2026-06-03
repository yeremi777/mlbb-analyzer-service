"""Pluggable AI chat providers (OpenRouter, OpenAI, Anthropic, etc.)."""

from app.analyzer.providers.base import ChatProvider, ProviderError, extract_json_text
from app.analyzer.providers.openrouter import OpenRouterChatProvider
from app.core.config import Settings

_NOT_IMPLEMENTED_PROVIDERS = frozenset({"openai", "anthropic"})

_SUPPORTED_PROVIDERS: dict[str, type[OpenRouterChatProvider]] = {
    "openrouter": OpenRouterChatProvider,
}


def create_chat_provider(settings: Settings) -> ChatProvider:
    slug = settings.ai_provider.strip().casefold()

    if slug in _NOT_IMPLEMENTED_PROVIDERS:
        raise ProviderError(f"AI provider '{settings.ai_provider}' is not implemented yet.")

    provider_cls = _SUPPORTED_PROVIDERS.get(slug)
    if provider_cls is None:
        supported = ", ".join(sorted(_SUPPORTED_PROVIDERS))
        raise ProviderError(
            f"Unsupported AI provider '{settings.ai_provider}'. Supported: {supported}."
        )
    if not settings.ai_enabled():
        raise ProviderError(f"AI provider '{settings.ai_provider}' is not configured.")
    return provider_cls(settings)


__all__ = [
    "ChatProvider",
    "OpenRouterChatProvider",
    "ProviderError",
    "create_chat_provider",
    "extract_json_text",
]
