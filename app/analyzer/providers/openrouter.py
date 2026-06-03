from typing import Any

from openrouter import OpenRouter
from openrouter.components.formatjsonobjectconfig import FormatJSONObjectConfig

from app.analyzer.providers.base import ProviderError, parse_json_object
from app.core.config import Settings

PROVIDER_NAME = "OpenRouter"


class OpenRouterChatProvider:
    provider_name = PROVIDER_NAME

    def __init__(self, settings: Settings) -> None:
        if not settings.openrouter_configured():
            raise ProviderError(f"{PROVIDER_NAME} API key is not configured.")
        self._model = settings.active_model()
        if not self._model:
            raise ProviderError("No model configured for the active AI provider.")

        self._api_key = settings.openrouter_api_key
        self._server_url = settings.openrouter_server_url
        self._http_referer = settings.openrouter_http_referer
        self._app_title = settings.openrouter_app_title
        self._timeout_ms = settings.ai_timeout_seconds * 1000

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, Any]:
        with OpenRouter(
            api_key=self._api_key,
            server_url=self._server_url,
            http_referer=self._http_referer,
            x_open_router_title=self._app_title,
            timeout_ms=self._timeout_ms,
        ) as client:
            response = client.chat.send(
                model=self._model,
                messages=messages,
                response_format=FormatJSONObjectConfig(type="json_object"),
                temperature=0.2,
                timeout_ms=self._timeout_ms,
            )
        content = _message_content(response)
        return parse_json_object(content, PROVIDER_NAME)

    def close(self) -> None:
        return None


def _message_content(response: Any) -> str:
    choices = getattr(response, "choices", None)
    if not choices:
        return ""
    first = choices[0]
    message = getattr(first, "message", None)
    if message is None:
        return ""
    content = getattr(message, "content", "")
    return content if isinstance(content, str) else str(content or "")
