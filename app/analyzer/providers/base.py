import json
import re
from typing import Any, Protocol, runtime_checkable

_JSON_FENCE_RE = re.compile(r"^```(?:json)?\s*\n?|\n?```\s*$", re.MULTILINE)


class ProviderError(RuntimeError):
    def __init__(
        self,
        message: str,
        *,
        retryable: bool = False,
        provider_name: str | None = None,
        status_code: int | None = None,
    ) -> None:
        self.retryable = retryable
        self.provider_name = provider_name
        self.status_code = status_code
        super().__init__(message)


def extract_json_text(raw: str) -> str:
    stripped = raw.strip()
    if stripped.startswith("```"):
        stripped = _JSON_FENCE_RE.sub("", stripped).strip()
    return stripped


def parse_json_object(content: str, provider_name: str) -> dict[str, Any]:
    if not content:
        raise ProviderError(f"{provider_name} returned an empty message.")

    try:
        parsed = json.loads(extract_json_text(content))
    except json.JSONDecodeError as exc:
        raise ProviderError(f"{provider_name} returned invalid JSON.") from exc

    if not isinstance(parsed, dict):
        raise ProviderError(f"{provider_name} JSON root must be an object.")
    return parsed


@runtime_checkable
class ChatProvider(Protocol):
    provider_name: str

    def complete_json(self, messages: list[dict[str, str]]) -> dict[str, Any]:
        ...

    def close(self) -> None:
        ...
