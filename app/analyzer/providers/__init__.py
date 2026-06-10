"""Pluggable AI chat providers (OpenRouter, OpenAI, Anthropic, etc.)."""

from app.analyzer.providers.base import ChatProvider, ProviderError, extract_json_text
from app.analyzer.providers.opencode_zen import OpenCodeZenChatProvider
from app.analyzer.providers.openrouter import OpenRouterChatProvider
from app.analyzer.providers.relay import RelayChatProvider
from app.core.config import Settings

_NOT_IMPLEMENTED_PROVIDERS = frozenset({"openai", "anthropic"})

_SUPPORTED_PROVIDERS: dict[str, type[ChatProvider]] = {
    "openrouter": OpenRouterChatProvider,
    "opencode_zen": OpenCodeZenChatProvider,
}


def create_chat_provider(settings: Settings) -> ChatProvider:
    slugs = settings.ai_provider_slugs()
    if not slugs:
        raise ProviderError("No AI providers configured.")

    providers: list[ChatProvider] = []
    for slug in slugs:
        if slug in _NOT_IMPLEMENTED_PROVIDERS:
            raise ProviderError(f"AI provider '{slug}' is not implemented yet.")

        provider_cls = _SUPPORTED_PROVIDERS.get(slug)
        if provider_cls is None:
            supported = ", ".join(sorted(_SUPPORTED_PROVIDERS))
            raise ProviderError(f"Unsupported AI provider '{slug}'. Supported: {supported}.")
        if not settings.provider_configured(slug):
            raise ProviderError(f"AI provider '{slug}' is not configured.")
        providers.append(provider_cls(settings))

    if len(providers) == 1:
        return providers[0]
    return RelayChatProvider(providers)


__all__ = [
    "ChatProvider",
    "OpenCodeZenChatProvider",
    "OpenRouterChatProvider",
    "ProviderError",
    "RelayChatProvider",
    "create_chat_provider",
    "extract_json_text",
]
