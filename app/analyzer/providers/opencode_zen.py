from typing import Any

import httpx

from app.analyzer.providers.base import ProviderError, parse_json_object
from app.core.config import Settings

PROVIDER_NAME = "OpenCode Zen"
_RETRYABLE_STATUS_CODES = frozenset({402, 408, 409, 425, 429})


class OpenCodeZenChatProvider:
    provider_name = PROVIDER_NAME

    def __init__(self, settings: Settings) -> None:
        if not settings.opencode_zen_configured():
            raise ProviderError(f"{PROVIDER_NAME} API key is not configured.")
        if settings.opencode_zen_endpoint_type != "chat_completions":
            raise ProviderError(
                f"{PROVIDER_NAME} endpoint type '{settings.opencode_zen_endpoint_type}' "
                "is not implemented yet."
            )

        model = settings.provider_model("opencode_zen")
        if not model:
            raise ProviderError(f"No model configured for {PROVIDER_NAME}.")

        self._model = _normalize_model(model)
        self._api_key = settings.opencode_zen_api_key
        self._server_url = settings.opencode_zen_server_url.rstrip("/")
        self._timeout_seconds = settings.ai_timeout_seconds

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, Any]:
        payload = {
            "model": self._model,
            "messages": messages,
            "response_format": {"type": "json_object"},
            "temperature": 0.2,
        }
        headers = {
            "Authorization": f"Bearer {self._api_key}",
            "Content-Type": "application/json",
        }

        try:
            response = httpx.post(
                f"{self._server_url}/chat/completions",
                headers=headers,
                json=payload,
                timeout=self._timeout_seconds,
            )
        except httpx.TimeoutException as exc:
            raise ProviderError(
                f"{PROVIDER_NAME} request timed out.",
                retryable=True,
                provider_name=PROVIDER_NAME,
            ) from exc
        except httpx.RequestError as exc:
            raise ProviderError(
                f"{PROVIDER_NAME} request failed: {exc}",
                retryable=True,
                provider_name=PROVIDER_NAME,
            ) from exc

        if response.status_code >= 400:
            raise _http_error(response)

        try:
            response_payload = response.json()
        except ValueError as exc:
            raise ProviderError(f"{PROVIDER_NAME} returned invalid JSON.") from exc

        content = _message_content(response_payload)
        return parse_json_object(content, PROVIDER_NAME)

    def close(self) -> None:
        return None


def _normalize_model(model: str) -> str:
    if model.startswith("opencode/"):
        return model.removeprefix("opencode/")
    return model


def _message_content(response: Any) -> str:
    choices = response.get("choices") if isinstance(response, dict) else None
    if not choices:
        return ""
    first = choices[0]
    message = first.get("message") if isinstance(first, dict) else None
    if message is None:
        return ""
    content = message.get("content", "") if isinstance(message, dict) else ""
    return content if isinstance(content, str) else str(content or "")


def _http_error(response: httpx.Response) -> ProviderError:
    detail = _error_detail(response)
    retryable = response.status_code in _RETRYABLE_STATUS_CODES or response.status_code >= 500
    return ProviderError(
        f"{PROVIDER_NAME} returned HTTP {response.status_code}: {detail}",
        retryable=retryable,
        provider_name=PROVIDER_NAME,
        status_code=response.status_code,
    )


def _error_detail(response: httpx.Response) -> str:
    try:
        payload = response.json()
    except ValueError:
        return response.text[:500] or response.reason_phrase

    if isinstance(payload, dict):
        error = payload.get("error")
        if isinstance(error, dict):
            message = error.get("message")
            if message:
                return str(message)
        if isinstance(error, str):
            return error
        message = payload.get("message")
        if message:
            return str(message)
    return str(payload)[:500]
