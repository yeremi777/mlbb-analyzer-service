package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeremi777/mlbb-analyzer-service/internal/analyzer"
	"github.com/yeremi777/mlbb-analyzer-service/internal/ratelimit"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
	"github.com/yeremi777/mlbb-analyzer-service/internal/store"
)

// Analyzer scores and explains matchups via an AI provider chain. Nil disables
// the analyze endpoints (they respond 504 ai_provider_not_configured).
type Analyzer interface {
	ScoreCounters(ctx context.Context, target staticdata.Hero, ms []store.HeroMatchup, language string) (*analyzer.ScoresResult, error)
	ScoreSynergies(ctx context.Context, anchor staticdata.Hero, ms []store.HeroMatchup, language string) (*analyzer.ScoresResult, error)
	CounterDetail(ctx context.Context, target staticdata.Hero, m store.HeroMatchup, language string) (*analyzer.DetailResult, error)
	SynergyDetail(ctx context.Context, anchor staticdata.Hero, m store.HeroMatchup, language string) (*analyzer.DetailResult, error)
	HasCachedScore(kind, heroID, language string) bool
	HasCachedDetail(kind, heroID, partnerID, language string) bool
}

type Server struct {
	db      store.Querier
	ai      Analyzer
	limiter *ratelimit.Limiter
}

// NewServer builds the gateway. ai may be nil (analyze endpoints answer 504);
// limiter may be nil (no rate limiting).
func NewServer(db store.Querier, ai Analyzer, limiter *ratelimit.Limiter) *Server {
	return &Server{db: db, ai: ai, limiter: limiter}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.root)
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/heroes", s.listHeroes)
	mux.HandleFunc("GET /api/heroes/{heroId}", s.getHero)
	mux.HandleFunc("GET /api/heroes/{heroId}/counters", s.listCounters)
	mux.HandleFunc("GET /api/heroes/{heroId}/synergies", s.listSynergies)
	mux.HandleFunc("POST /api/counters/analyze-score", s.analyzeCounterScore)
	mux.HandleFunc("POST /api/counters/analyze-detail", s.analyzeCounterDetail)
	mux.HandleFunc("POST /api/synergies/analyze-score", s.analyzeSynergyScore)
	mux.HandleFunc("POST /api/synergies/analyze-detail", s.analyzeSynergyDetail)
	s.docsRoutes(mux)
	return mux
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func intParam(raw string, fallback, lo, hi int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lo {
		return fallback
	}
	if n > hi {
		return hi
	}
	return n
}

func filterHeroes(heroes []staticdata.Hero, keep func(staticdata.Hero) bool) []staticdata.Hero {
	out := make([]staticdata.Hero, 0, len(heroes))
	for _, h := range heroes {
		if keep(h) {
			out = append(out, h)
		}
	}
	return out
}

func containsFold(values []string, want string) bool {
	for _, v := range values {
		if strings.ToLower(v) == want {
			return true
		}
	}
	return false
}

// root godoc
//
//	@Summary	Root
//	@Tags		health
//	@Produce	json
//	@Success	200	{string}	string	"Hello, World!"
//	@Router		/ [get]
func (s *Server) root(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, "Hello, World!")
}

// health godoc
//
//	@Summary		Health check
//	@Description	Lightweight service health status.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Router			/health [get]
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// WithCORS wraps the handler with credentialed CORS for the given origins.
func WithCORS(origins map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origins[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
