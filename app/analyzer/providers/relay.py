from typing import Any

from app.analyzer.providers.base import ChatProvider, ProviderError

PROVIDER_NAME = "Relay"


class RelayChatProvider:
    provider_name = PROVIDER_NAME

    def __init__(self, providers: list[ChatProvider]) -> None:
        if not providers:
            raise ProviderError("AI relay has no configured providers.")
        self._providers = providers

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, Any]:
        errors: list[str] = []
        for provider in self._providers:
            try:
                return provider.complete_json(messages)
            except ProviderError as exc:
                errors.append(f"{provider.provider_name}: {exc}")
                if not exc.retryable:
                    raise

        joined_errors = " | ".join(errors)
        raise ProviderError(f"All AI relay providers failed: {joined_errors}")

    def close(self) -> None:
        for provider in self._providers:
            provider.close()
