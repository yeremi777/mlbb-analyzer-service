import hashlib
import hmac
from functools import lru_cache
from uuid import UUID, uuid4

from fastapi import HTTPException, Request, Response
from redis import Redis
from redis.exceptions import RedisError

from app.core.config import Settings

_RATE_LIMIT_PREFIX = "rate:analyze"
_CHECK_AND_INCREMENT_SCRIPT = """
local count = redis.call("GET", KEYS[1])
if count and tonumber(count) >= tonumber(ARGV[1]) then
    return {0, tonumber(count), redis.call("TTL", KEYS[1])}
end

count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[2])
end

return {1, count, redis.call("TTL", KEYS[1])}
"""


@lru_cache
def _redis_client(redis_url: str) -> Redis:
    return Redis.from_url(redis_url, decode_responses=True, socket_timeout=2)


def _client_ip(request: Request) -> str:
    forwarded_for = request.headers.get("x-forwarded-for")
    if forwarded_for:
        return forwarded_for.split(",", maxsplit=1)[0].strip()
    if request.client is not None:
        return request.client.host
    return "unknown"


def _hash_ip(ip_address: str, salt: str) -> str:
    digest = hmac.new(
        salt.encode("utf-8"),
        ip_address.encode("utf-8"),
        hashlib.sha256,
    ).hexdigest()
    return digest


def _client_id_from_cookie(request: Request, settings: Settings) -> str | None:
    client_id = request.cookies.get(settings.rate_limit_cookie_name)
    if not client_id:
        return None
    try:
        return str(UUID(client_id))
    except ValueError:
        return None


def _set_client_cookie(response: Response, client_id: str, settings: Settings) -> None:
    response.set_cookie(
        key=settings.rate_limit_cookie_name,
        value=client_id,
        max_age=settings.rate_limit_cookie_max_age_seconds,
        httponly=True,
        secure=settings.rate_limit_cookie_secure,
        samesite=settings.rate_limit_cookie_samesite,
    )


def _increment_or_raise(
    client: Redis,
    key: str,
    max_requests: int,
    window_seconds: int,
) -> None:
    allowed, _count, ttl = client.eval(
        _CHECK_AND_INCREMENT_SCRIPT,
        1,
        key,
        max_requests,
        window_seconds,
    )
    if not allowed:
        detail = {
            "code": "rate_limit_exceeded",
            "message": "Too many analyze requests. Please try again later.",
        }
        headers = {"Retry-After": str(ttl)} if ttl > 0 else None
        raise HTTPException(status_code=429, detail=detail, headers=headers)


def _max_requests_for_endpoint(settings: Settings, endpoint_name: str) -> int:
    if endpoint_name.endswith("-detail"):
        return settings.rate_limit_analyze_max_requests * settings.rate_limit_analyze_detail_multiplier
    return settings.rate_limit_analyze_max_requests


def enforce_analyze_rate_limit(
    request: Request,
    response: Response,
    settings: Settings,
    endpoint_name: str,
) -> None:
    if not settings.rate_limit_enabled:
        return
    redis_url = settings.redis_url()
    if redis_url is None:
        raise HTTPException(
            status_code=503,
            detail={
                "code": "rate_limit_not_configured",
                "message": "Rate limiting is enabled but Redis is not configured.",
            },
        )

    ip_hash = _hash_ip(_client_ip(request), settings.rate_limit_salt)
    ip_key = f"{_RATE_LIMIT_PREFIX}:{endpoint_name}:ip:{ip_hash}"

    try:
        client = _redis_client(redis_url)
        max_requests = _max_requests_for_endpoint(settings, endpoint_name)
        _increment_or_raise(
            client,
            ip_key,
            max_requests,
            settings.rate_limit_analyze_window_seconds,
        )

        client_id = _client_id_from_cookie(request, settings)
        if client_id is None:
            client_id = str(uuid4())
            _set_client_cookie(response, client_id, settings)

        client_key = f"{_RATE_LIMIT_PREFIX}:{endpoint_name}:client:{client_id}"
        _increment_or_raise(
            client,
            client_key,
            max_requests,
            settings.rate_limit_analyze_window_seconds,
        )
    except HTTPException:
        raise
    except RedisError as exc:
        raise HTTPException(
            status_code=503,
            detail={
                "code": "rate_limit_unavailable",
                "message": "Rate limit storage is unavailable. Please try again later.",
            },
        ) from exc
