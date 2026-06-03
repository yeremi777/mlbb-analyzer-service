class AnalyzerError(Exception):
    def __init__(self, code: str, message: str) -> None:
        self.code = code
        self.message = message
        super().__init__(message)


class AnalyzerNotConfiguredError(AnalyzerError):
    def __init__(self, provider: str) -> None:
        super().__init__(
            "ai_provider_not_configured",
            f"AI provider '{provider}' is not configured. Set the required API key in .env.",
        )


class AnalyzerNotImplementedError(AnalyzerError):
    def __init__(self, provider: str) -> None:
        super().__init__(
            "ai_provider_not_implemented",
            f"AI provider '{provider}' is not implemented yet.",
        )


class AnalyzerProviderError(AnalyzerError):
    def __init__(self, message: str) -> None:
        super().__init__("ai_provider_error", message)
