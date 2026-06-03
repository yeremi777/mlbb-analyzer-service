"""OpenAPI response examples for counter analyze endpoints."""

from app.schemas.analysis import ErrorResponse

_COUNTER_SCORE_404_EXAMPLES = {
    "target_hero_not_found": {
        "summary": "Target hero not found",
        "value": {
            "error": {
                "code": "target_hero_not_found",
                "message": "Target hero was not found in the dataset.",
            }
        },
    },
    "counter_data_not_found": {
        "summary": "Counter data not found",
        "value": {
            "error": {
                "code": "counter_data_not_found",
                "message": "Counter data was not found for the target hero.",
            }
        },
    },
}

_COUNTER_DETAIL_404_EXAMPLES = {
    **_COUNTER_SCORE_404_EXAMPLES,
    "counter_hero_not_found": {
        "summary": "Counter hero not found",
        "value": {
            "error": {
                "code": "counter_hero_not_found",
                "message": "Counter hero was not found in the dataset.",
            }
        },
    },
    "counter_matchup_not_found": {
        "summary": "Counter matchup not found",
        "value": {
            "error": {
                "code": "counter_matchup_not_found",
                "message": "Counter matchup was not found for the target hero.",
            }
        },
    },
}

_AI_501_EXAMPLE = {
    "error": {
        "code": "ai_provider_not_implemented",
        "message": "AI provider is not implemented yet.",
    }
}

_AI_502_EXAMPLE = {
    "error": {
        "code": "ai_provider_error",
        "message": "AI provider returned invalid JSON.",
    }
}

_AI_504_EXAMPLE = {
    "error": {
        "code": "ai_provider_not_configured",
        "message": "AI provider is not configured. Set the required API key in .env.",
    }
}


def _json_content(examples: dict | None = None, example: dict | None = None) -> dict:
    body: dict = {}
    if examples is not None:
        body["examples"] = examples
    if example is not None:
        body["example"] = example
    return {"application/json": body}


COUNTER_ANALYZE_SCORE_RESPONSES: dict[int, dict] = {
    404: {
        "model": ErrorResponse,
        "description": "Target hero or counter data was not found.",
        "content": _json_content(examples=_COUNTER_SCORE_404_EXAMPLES),
    },
    501: {
        "model": ErrorResponse,
        "description": "AI provider is not implemented.",
        "content": _json_content(example=_AI_501_EXAMPLE),
    },
    502: {
        "model": ErrorResponse,
        "description": "AI provider returned an error or invalid output.",
        "content": _json_content(example=_AI_502_EXAMPLE),
    },
    504: {
        "model": ErrorResponse,
        "description": "AI provider is not configured or timed out.",
        "content": _json_content(example=_AI_504_EXAMPLE),
    },
}

COUNTER_ANALYZE_DETAIL_RESPONSES: dict[int, dict] = {
    404: {
        "model": ErrorResponse,
        "description": "Target hero, counter hero, or matchup was not found.",
        "content": _json_content(examples=_COUNTER_DETAIL_404_EXAMPLES),
    },
    501: {
        "model": ErrorResponse,
        "description": "AI provider is not implemented.",
        "content": _json_content(example=_AI_501_EXAMPLE),
    },
    502: {
        "model": ErrorResponse,
        "description": "AI provider returned an error or invalid output.",
        "content": _json_content(example=_AI_502_EXAMPLE),
    },
    504: {
        "model": ErrorResponse,
        "description": "AI provider is not configured or timed out.",
        "content": _json_content(example=_AI_504_EXAMPLE),
    },
}
