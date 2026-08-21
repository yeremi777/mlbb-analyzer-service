package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/yeremi777/mlbb-analyzer-service/internal/analyzer"
	"github.com/yeremi777/mlbb-analyzer-service/internal/api"
	"github.com/yeremi777/mlbb-analyzer-service/internal/config"
	"github.com/yeremi777/mlbb-analyzer-service/internal/ratelimit"
)

func allowedOrigins() map[string]bool {
	origins := map[string]bool{"http://localhost:3000": true}
	for _, o := range strings.Split(os.Getenv("FRONTEND_ORIGIN"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins[o] = true
		}
	}
	return origins
}

// buildAnalyzer assembles the provider chain from AI_PROVIDERS. A provider
// with a missing key is skipped with a warning; no usable provider yields nil,
// which surfaces as 504 ai_provider_not_configured on the analyze endpoints.
func buildAnalyzer() *analyzer.Analyzer {
	timeout := time.Duration(config.IntEnv("AI_TIMEOUT_SECONDS", 20)) * time.Second
	var providers []analyzer.Provider
	slugs := strings.Split(config.Getenv("AI_PROVIDERS", config.Getenv("AI_PROVIDER", "openrouter")), ",")
	for _, slug := range slugs {
		var (
			p   analyzer.Provider
			err error
		)
		switch strings.ToLower(strings.TrimSpace(slug)) {
		case "openrouter":
			p, err = analyzer.NewChatProvider(analyzer.ChatProviderConfig{
				Name:      "OpenRouter",
				ServerURL: config.Getenv("OPENROUTER_SERVER_URL", "https://openrouter.ai/api/v1"),
				APIKey:    realKey(os.Getenv("OPENROUTER_API_KEY")),
				Model:     config.Getenv("OPENROUTER_MODEL", "openrouter/free"),
				Timeout:   timeout,
				ExtraHeaders: map[string]string{
					"HTTP-Referer":       config.Getenv("OPENROUTER_HTTP_REFERER", "http://127.0.0.1:8000"),
					"X-OpenRouter-Title": config.Getenv("OPENROUTER_APP_TITLE", "MLBB Analyzer Service"),
				},
			})
		case "opencode_zen":
			p, err = analyzer.NewChatProvider(analyzer.ChatProviderConfig{
				Name:      "OpenCode Zen",
				ServerURL: config.Getenv("OPENCODE_ZEN_SERVER_URL", "https://opencode.ai/zen/v1"),
				APIKey:    realKey(os.Getenv("OPENCODE_ZEN_API_KEY")),
				Model:     strings.TrimPrefix(os.Getenv("OPENCODE_ZEN_MODEL"), "opencode/"),
				Timeout:   timeout,
			})
		default:
			continue
		}
		if err != nil {
			slog.Warn("skipping AI provider", "provider", slug, "reason", err)
			continue
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		return nil
	}
	chain := providers[0]
	if len(providers) > 1 {
		chain = analyzer.NewRelay(providers...)
	}
	return analyzer.New(chain, analyzer.Config{
		CacheTTL:        time.Duration(config.IntEnv("AI_ANALYSIS_CACHE_TTL_SECONDS", 600)) * time.Second,
		CacheMaxEntries: config.IntEnv("AI_ANALYSIS_CACHE_MAX_ENTRIES", 256),
	})
}

// realKey filters the placeholder values .env.example ships with.
func realKey(key string) string {
	if strings.HasPrefix(key, "<") {
		return ""
	}
	return key
}

// buildLimiter assembles the Redis rate limiter from RATE_LIMIT_* / REDIS_*.
// Returns nil when disabled; enabled-but-unconfigured Redis is a startup
// error rather than a silent no-op.
func buildLimiter() (*ratelimit.Limiter, error) {
	if config.Getenv("RATE_LIMIT_ENABLED", "false") != "true" {
		return nil, nil
	}
	addr := os.Getenv("REDIS_URL")
	var client *redis.Client
	if addr != "" {
		opts, err := redis.ParseURL(addr)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		client = redis.NewClient(opts)
	} else {
		host := os.Getenv("REDIS_HOST")
		if host == "" {
			return nil, fmt.Errorf("rate limiting is enabled but Redis is not configured")
		}
		client = redis.NewClient(&redis.Options{
			Addr: host + ":" + config.Getenv("REDIS_PORT", "6379"),
			DB:   config.IntEnv("REDIS_DB", 0),
		})
	}
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(config.Getenv("RATE_LIMIT_COOKIE_SAMESITE", "lax")) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	}
	return ratelimit.New(client, ratelimit.Config{
		Enabled:          true,
		MaxRequests:      config.IntEnv("RATE_LIMIT_ANALYZE_MAX_REQUESTS", 5),
		WindowSeconds:    config.IntEnv("RATE_LIMIT_ANALYZE_WINDOW_SECONDS", 18000),
		DetailMultiplier: config.IntEnv("RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER", 3),
		CookieName:       config.Getenv("RATE_LIMIT_COOKIE_NAME", "mlbb_analyzer_client_id"),
		CookieMaxAge:     config.IntEnv("RATE_LIMIT_COOKIE_MAX_AGE_SECONDS", 2_592_000),
		CookieSecure:     config.Getenv("RATE_LIMIT_COOKIE_SECURE", "false") == "true",
		CookieSameSite:   sameSite,
		Salt:             config.Getenv("RATE_LIMIT_SALT", "mlbb-analyzer-service-local-rate-limit"),
	}), nil
}

func run() error {
	_ = godotenv.Load()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, config.DatabaseURL())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	var ai api.Analyzer
	if a := buildAnalyzer(); a != nil {
		ai = a
	}
	limiter, err := buildLimiter()
	if err != nil {
		return err
	}
	addr := config.Getenv("API_HOST", "127.0.0.1") + ":" + config.Getenv("API_PORT", "8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: api.WithCORS(allowedOrigins(), api.NewServer(pool, ai, limiter).Handler()),
	}

	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("api listening", "addr", addr)

	select {
	case err := <-errCh:
		return err
	case <-shutdownCtx.Done():
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("api stopped")
	return nil
}

// @title			MLBB Analyzer Service
// @version		1.0
// @description	Serves the MLBB hero, counter, and synergy dataset from Postgres and returns AI-scored counter recommendations for frontend consumption.
// @BasePath		/
//
// @tag.name			health
// @tag.description	Service health endpoints.
// @tag.name			heroes
// @tag.description	Hero catalog, counter, and synergy matchup endpoints.
// @tag.name			counters
// @tag.description	AI scoring and detail endpoints for counter matchups.
// @tag.name			synergies
// @tag.description	AI scoring and detail endpoints for synergy pairings.
func main() {
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api failed", "err", err)
		os.Exit(1)
	}
}
