import httpx
import pytest

from app.analyzer.providers import ProviderError, create_chat_provider
from app.analyzer.providers.opencode_zen import OpenCodeZenChatProvider
from app.analyzer.providers.openrouter import OpenRouterChatProvider
from app.analyzer.providers.relay import RelayChatProvider
from app.core.config import Settings


def test_create_chat_provider_returns_openrouter() -> None:
    settings = Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
    )
    provider = create_chat_provider(settings)
    assert provider.provider_name == "OpenRouter"
    provider.close()


def test_create_chat_provider_returns_relay_for_multiple_providers() -> None:
    settings = Settings(
        AI_PROVIDERS="openrouter,opencode_zen",
        OPENROUTER_API_KEY="test-key",
        OPENCODE_ZEN_API_KEY="test-key",
        OPENCODE_ZEN_MODEL="deepseek-v4-flash",
    )
    provider = create_chat_provider(settings)
    assert isinstance(provider, RelayChatProvider)
    provider.close()


def test_settings_parses_ai_provider_list() -> None:
    settings = Settings(AI_PROVIDERS="openrouter, opencode_zen")

    assert settings.ai_provider_slugs() == ["openrouter", "opencode_zen"]


def test_relay_falls_back_on_retryable_provider_error() -> None:
    class FailingProvider:
        provider_name = "Failing"

        def complete_json(self, messages: list[dict[str, str]]) -> dict[str, object]:
            raise ProviderError("rate limited", retryable=True)

        def close(self) -> None:
            return None

    class WorkingProvider:
        provider_name = "Working"

        def complete_json(self, messages: list[dict[str, str]]) -> dict[str, object]:
            return {"ok": True}

        def close(self) -> None:
            return None

    relay = RelayChatProvider([FailingProvider(), WorkingProvider()])

    assert relay.complete_json([{"role": "user", "content": "Return JSON."}]) == {"ok": True}


def test_create_chat_provider_rejects_unknown_provider() -> None:
    settings = Settings(AI_PROVIDER="anthropic", OPENROUTER_API_KEY="test-key")
    with pytest.raises(ProviderError, match="not implemented"):
        create_chat_provider(settings)


def test_create_chat_provider_returns_opencode_zen() -> None:
    settings = Settings(
        AI_PROVIDERS="opencode_zen",
        OPENCODE_ZEN_API_KEY="test-key",
        OPENCODE_ZEN_MODEL="deepseek-v4-flash",
    )
    provider = create_chat_provider(settings)
    assert provider.provider_name == "OpenCode Zen"
    provider.close()


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


def test_openrouter_complete_json_uses_httpx(monkeypatch: pytest.MonkeyPatch) -> None:
    captured: dict[str, object] = {}

    def fake_post(url: str, **kwargs: object) -> httpx.Response:
        captured["url"] = url
        captured.update(kwargs)
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": '{"recommendations":[]}',
                        }
                    }
                ]
            },
        )

    monkeypatch.setattr("app.analyzer.providers.openrouter.httpx.post", fake_post)
    settings = Settings(
        AI_PROVIDER="openrouter",
        OPENROUTER_API_KEY="test-key",
        OPENROUTER_MODEL="openrouter/free",
    )
    provider = OpenRouterChatProvider(settings)

    payload = provider.complete_json([{"role": "user", "content": "Return JSON."}])

    assert payload == {"recommendations": []}
    assert captured["url"] == "https://openrouter.ai/api/v1/chat/completions"
    assert captured["json"] == {
        "model": "openrouter/free",
        "messages": [{"role": "user", "content": "Return JSON."}],
        "response_format": {"type": "json_object"},
        "temperature": 0.2,
    }


def test_opencode_zen_complete_json_uses_httpx(monkeypatch: pytest.MonkeyPatch) -> None:
    captured: dict[str, object] = {}

    def fake_post(url: str, **kwargs: object) -> httpx.Response:
        captured["url"] = url
        captured.update(kwargs)
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": '{"recommendations":[]}',
                        }
                    }
                ]
            },
        )

    monkeypatch.setattr("app.analyzer.providers.opencode_zen.httpx.post", fake_post)
    settings = Settings(
        AI_PROVIDERS="opencode_zen",
        OPENCODE_ZEN_API_KEY="test-key",
        OPENCODE_ZEN_MODEL="deepseek-v4-flash",
    )
    provider = OpenCodeZenChatProvider(settings)

    payload = provider.complete_json([{"role": "user", "content": "Return JSON."}])

    assert payload == {"recommendations": []}
    assert captured["url"] == "https://opencode.ai/zen/v1/chat/completions"
    assert captured["json"] == {
        "model": "deepseek-v4-flash",
        "messages": [{"role": "user", "content": "Return JSON."}],
        "response_format": {"type": "json_object"},
        "temperature": 0.2,
    }


def test_opencode_zen_strips_opencode_model_prefix(monkeypatch: pytest.MonkeyPatch) -> None:
    captured: dict[str, object] = {}

    def fake_post(url: str, **kwargs: object) -> httpx.Response:
        captured.update(kwargs)
        return httpx.Response(
            200,
            json={"choices": [{"message": {"content": '{"recommendations":[]}'}}]},
        )

    monkeypatch.setattr("app.analyzer.providers.opencode_zen.httpx.post", fake_post)
    settings = Settings(
        AI_PROVIDERS="opencode_zen",
        OPENCODE_ZEN_API_KEY="test-key",
        OPENCODE_ZEN_MODEL="opencode/deepseek-v4-flash",
    )
    provider = OpenCodeZenChatProvider(settings)

    provider.complete_json([{"role": "user", "content": "Return JSON."}])

    assert captured["json"]["model"] == "deepseek-v4-flash"
