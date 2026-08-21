package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/analyzer"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
	"github.com/yeremi777/mlbb-analyzer-service/internal/postgres"
	"github.com/yeremi777/mlbb-analyzer-service/internal/ratelimit"
)

func writeAnalyzerError(w http.ResponseWriter, err error) {
	var ae *analyzer.Error
	if !errors.As(err, &ae) {
		writeError(w, http.StatusBadGateway, "ai_provider_error", err.Error())
		return
	}
	status := http.StatusGatewayTimeout
	switch ae.Code {
	case "ai_provider_not_implemented":
		status = http.StatusNotImplemented
	case "ai_provider_error":
		status = http.StatusBadGateway
	}
	writeError(w, status, ae.Code, ae.Message)
}

// enforceRateLimit counts the request unless the analysis is already cached
// (a cache hit costs nothing, so it consumes no quota). Writes the error
// response and returns false when the request must not proceed.
func (s *Server) enforceRateLimit(w http.ResponseWriter, r *http.Request, cached bool, endpointName string) bool {
	if s.limiter == nil || cached {
		return true
	}
	err := s.limiter.Enforce(w, r, endpointName)
	if err == nil {
		return true
	}
	if le, ok := err.(*ratelimit.Error); ok {
		if le.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(le.RetryAfter))
		}
		writeError(w, le.Status, le.Code, le.Message)
	} else {
		writeError(w, http.StatusServiceUnavailable, "rate_limit_unavailable", err.Error())
	}
	return false
}

func (s *Server) requireAnalyzer(w http.ResponseWriter) bool {
	if s.ai == nil {
		writeError(w, http.StatusGatewayTimeout, "ai_provider_not_configured",
			"AI provider is not configured. Set the required API key in .env.")
		return false
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "Request body is not valid JSON.")
		return false
	}
	return true
}

func normalizeLanguage(w http.ResponseWriter, language string) (string, bool) {
	switch language {
	case "":
		return "en", true
	case "en", "id":
		return language, true
	}
	writeError(w, http.StatusUnprocessableEntity, "invalid_request", "language must be 'en' or 'id'.")
	return "", false
}

// analyzeContext validates hero + matchup existence with the endpoint-specific
// 404 codes and returns the hero with its matchups.
func (s *Server) analyzeContext(
	w http.ResponseWriter, r *http.Request, heroID string,
	fetch func(context.Context, postgres.Querier, string) ([]domain.HeroMatchup, error),
	heroCode, dataCode, dataMessage string,
) (domain.Hero, []domain.HeroMatchup, bool) {
	hero, err := postgres.GetHero(r.Context(), s.db, heroID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, heroCode, "Hero was not found in the dataset.")
		return hero, nil, false
	} else if err != nil {
		s.internalError(w, "get hero", err)
		return hero, nil, false
	}
	ms, err := fetch(r.Context(), s.db, heroID)
	if err != nil {
		s.internalError(w, "list matchups", err)
		return hero, nil, false
	}
	if len(ms) == 0 {
		writeError(w, http.StatusNotFound, dataCode, dataMessage)
		return hero, nil, false
	}
	return hero, ms, true
}

// analyzeCounterScore godoc
//
//	@Summary		Score all counter matchups for one target hero
//	@Description	Returns AI-produced score and confidence for every counter matchup of the target hero, ranked best-first. Cached results do not consume rate-limit quota.
//	@Tags			counters
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AnalyzeCounterScoreRequest	true	"Target hero and output language"
//	@Success		200		{object}	AnalyzeCounterScoreResponse
//	@Failure		404		{object}	ErrorResponse	"target_hero_not_found, counter_data_not_found"
//	@Failure		422		{object}	ErrorResponse	"invalid_request"
//	@Failure		429		{object}	ErrorResponse	"rate_limit_exceeded"
//	@Failure		502		{object}	ErrorResponse	"ai_provider_error"
//	@Failure		503		{object}	ErrorResponse	"rate_limit_unavailable"
//	@Failure		504		{object}	ErrorResponse	"ai_provider_not_configured"
//	@Router			/api/counters/analyze-score [post]
func (s *Server) analyzeCounterScore(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeCounterScoreRequest
	if !decodeBody(w, r, &req) || !s.requireAnalyzer(w) {
		return
	}
	language, ok := normalizeLanguage(w, req.Language)
	if !ok {
		return
	}
	hero, ms, ok := s.analyzeContext(w, r, req.TargetHeroID, postgres.CountersForTarget,
		"target_hero_not_found", "counter_data_not_found", "Counter data was not found for the target hero.")
	if !ok {
		return
	}
	if !s.enforceRateLimit(w, r, s.ai.HasCachedScore("counter", req.TargetHeroID, language), "analyze-counter-score") {
		return
	}
	result, err := s.ai.ScoreCounters(r.Context(), hero, ms, language)
	if err != nil {
		writeAnalyzerError(w, err)
		return
	}
	recs := make([]CounterScoreRecommendation, len(result.Recommendations))
	for i, item := range result.Recommendations {
		recs[i] = CounterScoreRecommendation{
			Rank: item.Rank, CounterHeroID: item.CounterHeroID,
			Score: item.Score, Confidence: item.Confidence,
		}
	}
	writeJSON(w, http.StatusOK, AnalyzeCounterScoreResponse{
		TargetHeroID: req.TargetHeroID, Source: "ai", Recommendations: recs,
	})
}

// analyzeSynergyScore godoc
//
//	@Summary		Score all synergy pairings for one anchor hero
//	@Description	Returns AI-produced score and confidence for every synergy pairing of the anchor hero, ranked best-first. Cached results do not consume rate-limit quota.
//	@Tags			synergies
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AnalyzeSynergyScoreRequest	true	"Anchor hero and output language"
//	@Success		200		{object}	AnalyzeSynergyScoreResponse
//	@Failure		404		{object}	ErrorResponse	"anchor_hero_not_found, synergy_data_not_found"
//	@Failure		422		{object}	ErrorResponse	"invalid_request"
//	@Failure		429		{object}	ErrorResponse	"rate_limit_exceeded"
//	@Failure		502		{object}	ErrorResponse	"ai_provider_error"
//	@Failure		503		{object}	ErrorResponse	"rate_limit_unavailable"
//	@Failure		504		{object}	ErrorResponse	"ai_provider_not_configured"
//	@Router			/api/synergies/analyze-score [post]
func (s *Server) analyzeSynergyScore(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeSynergyScoreRequest
	if !decodeBody(w, r, &req) || !s.requireAnalyzer(w) {
		return
	}
	language, ok := normalizeLanguage(w, req.Language)
	if !ok {
		return
	}
	hero, ms, ok := s.analyzeContext(w, r, req.AnchorHeroID, postgres.SynergiesForAnchor,
		"anchor_hero_not_found", "synergy_data_not_found", "Synergy data was not found for the anchor hero.")
	if !ok {
		return
	}
	if !s.enforceRateLimit(w, r, s.ai.HasCachedScore("synergy", req.AnchorHeroID, language), "analyze-synergy-score") {
		return
	}
	result, err := s.ai.ScoreSynergies(r.Context(), hero, ms, language)
	if err != nil {
		writeAnalyzerError(w, err)
		return
	}
	recs := make([]SynergyScoreRecommendation, len(result.Recommendations))
	for i, item := range result.Recommendations {
		recs[i] = SynergyScoreRecommendation{
			Rank: item.Rank, SynergyHeroID: item.CounterHeroID,
			Score: item.Score, Confidence: item.Confidence,
		}
	}
	writeJSON(w, http.StatusOK, AnalyzeSynergyScoreResponse{
		AnchorHeroID: req.AnchorHeroID, Source: "ai", Recommendations: recs,
	})
}

func findMatchup(ms []domain.HeroMatchup, partnerID string) (domain.HeroMatchup, bool) {
	for _, m := range ms {
		if m.Second.UID == partnerID {
			return m, true
		}
	}
	return domain.HeroMatchup{}, false
}

// analyzeCounterDetail godoc
//
//	@Summary		Explain one counter matchup in detail
//	@Description	Returns summary, strengths, conditions, and failure cases for one target/counter pair, grounded only in authored proof evidence.
//	@Tags			counters
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AnalyzeCounterDetailRequest	true	"Target hero, counter hero, and output language"
//	@Success		200		{object}	AnalyzeCounterDetailResponse
//	@Failure		404		{object}	ErrorResponse	"target_hero_not_found, counter_hero_not_found, counter_matchup_not_found"
//	@Failure		422		{object}	ErrorResponse	"invalid_request"
//	@Failure		429		{object}	ErrorResponse	"rate_limit_exceeded"
//	@Failure		502		{object}	ErrorResponse	"ai_provider_error"
//	@Failure		503		{object}	ErrorResponse	"rate_limit_unavailable"
//	@Failure		504		{object}	ErrorResponse	"ai_provider_not_configured"
//	@Router			/api/counters/analyze-detail [post]
func (s *Server) analyzeCounterDetail(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeCounterDetailRequest
	if !decodeBody(w, r, &req) || !s.requireAnalyzer(w) {
		return
	}
	language, ok := normalizeLanguage(w, req.Language)
	if !ok {
		return
	}
	hero, ms, ok := s.analyzeContext(w, r, req.TargetHeroID, postgres.CountersForTarget,
		"target_hero_not_found", "counter_data_not_found", "Counter data was not found for the target hero.")
	if !ok {
		return
	}
	if _, err := postgres.GetHero(r.Context(), s.db, req.CounterHeroID); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "counter_hero_not_found", "Counter hero was not found in the dataset.")
		return
	} else if err != nil {
		s.internalError(w, "get counter hero", err)
		return
	}
	m, found := findMatchup(ms, req.CounterHeroID)
	if !found {
		writeError(w, http.StatusNotFound, "counter_matchup_not_found", "Counter matchup was not found for the target hero.")
		return
	}
	if !s.enforceRateLimit(w, r, s.ai.HasCachedDetail("counter", req.TargetHeroID, req.CounterHeroID, language), "analyze-counter-detail") {
		return
	}
	result, err := s.ai.CounterDetail(r.Context(), hero, m, language)
	if err != nil {
		writeAnalyzerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, AnalyzeCounterDetailResponse{
		TargetHeroID: req.TargetHeroID, CounterHeroID: req.CounterHeroID, Source: "ai",
		Score: result.Score, Confidence: result.Confidence, Summary: result.Summary,
		Strengths: result.Strengths, Conditions: result.Conditions,
		FailureCases: result.FailureCases, EvidenceIDs: result.EvidenceIDs,
	})
}

// analyzeSynergyDetail godoc
//
//	@Summary		Explain one synergy pairing in detail
//	@Description	Returns summary, strengths, conditions, and failure cases for one anchor/synergy pair, grounded only in authored proof evidence.
//	@Tags			synergies
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AnalyzeSynergyDetailRequest	true	"Anchor hero, synergy hero, and output language"
//	@Success		200		{object}	AnalyzeSynergyDetailResponse
//	@Failure		404		{object}	ErrorResponse	"anchor_hero_not_found, synergy_hero_not_found, synergy_matchup_not_found"
//	@Failure		422		{object}	ErrorResponse	"invalid_request"
//	@Failure		429		{object}	ErrorResponse	"rate_limit_exceeded"
//	@Failure		502		{object}	ErrorResponse	"ai_provider_error"
//	@Failure		503		{object}	ErrorResponse	"rate_limit_unavailable"
//	@Failure		504		{object}	ErrorResponse	"ai_provider_not_configured"
//	@Router			/api/synergies/analyze-detail [post]
func (s *Server) analyzeSynergyDetail(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeSynergyDetailRequest
	if !decodeBody(w, r, &req) || !s.requireAnalyzer(w) {
		return
	}
	language, ok := normalizeLanguage(w, req.Language)
	if !ok {
		return
	}
	hero, ms, ok := s.analyzeContext(w, r, req.AnchorHeroID, postgres.SynergiesForAnchor,
		"anchor_hero_not_found", "synergy_data_not_found", "Synergy data was not found for the anchor hero.")
	if !ok {
		return
	}
	if _, err := postgres.GetHero(r.Context(), s.db, req.SynergyHeroID); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "synergy_hero_not_found", "Synergy hero was not found in the dataset.")
		return
	} else if err != nil {
		s.internalError(w, "get synergy hero", err)
		return
	}
	m, found := findMatchup(ms, req.SynergyHeroID)
	if !found {
		writeError(w, http.StatusNotFound, "synergy_matchup_not_found", "Synergy matchup was not found for the anchor hero.")
		return
	}
	if !s.enforceRateLimit(w, r, s.ai.HasCachedDetail("synergy", req.AnchorHeroID, req.SynergyHeroID, language), "analyze-synergy-detail") {
		return
	}
	result, err := s.ai.SynergyDetail(r.Context(), hero, m, language)
	if err != nil {
		writeAnalyzerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, AnalyzeSynergyDetailResponse{
		AnchorHeroID: req.AnchorHeroID, SynergyHeroID: req.SynergyHeroID, Source: "ai",
		Score: result.Score, Confidence: result.Confidence, Summary: result.Summary,
		Strengths: result.Strengths, Conditions: result.Conditions,
		FailureCases: result.FailureCases, EvidenceIDs: result.EvidenceIDs,
	})
}
