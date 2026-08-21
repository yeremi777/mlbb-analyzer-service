package api

import "github.com/yeremi777/mlbb-analyzer-service/internal/domain"

// ErrorBody is the payload of every non-2xx response.
type ErrorBody struct {
	Code    string `json:"code" example:"hero_not_found"`
	Message string `json:"message" example:"Hero was not found in the dataset."`
}

// ErrorResponse wraps ErrorBody under a single "error" key.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// HeroListResponse is one page of the hero catalog.
type HeroListResponse struct {
	Items []domain.Hero `json:"items"`
	Page  int           `json:"page" example:"1"`
	Size  int           `json:"size" example:"10"`
	Total int           `json:"total" example:"132"`
	Pages int           `json:"pages" example:"14"`
}

// CounterMatchup is one authored counter relation with the counter hero joined in.
type CounterMatchup struct {
	TargetHeroID string         `json:"targetHeroId" example:"tigreal"`
	CounterHero  domain.Hero    `json:"counterHero"`
	Reasons      []string       `json:"reasons"`
	CounterTypes []string       `json:"counterTypes"`
	Proof        []domain.Proof `json:"proof"`
}

// SynergyMatchup is one authored synergy pairing with the partner hero joined in.
type SynergyMatchup struct {
	AnchorHeroID string         `json:"anchorHeroId" example:"tigreal"`
	SynergyHero  domain.Hero    `json:"synergyHero"`
	Reasons      []string       `json:"reasons"`
	SynergyTypes []string       `json:"synergyTypes"`
	Proof        []domain.Proof `json:"proof"`
}

// AnalyzeCounterScoreRequest asks for AI scores across every counter of one target hero.
type AnalyzeCounterScoreRequest struct {
	TargetHeroID string `json:"targetHeroId" example:"tigreal"`
	Language     string `json:"language" enums:"en,id" example:"en"`
}

// AnalyzeSynergyScoreRequest asks for AI scores across every synergy of one anchor hero.
type AnalyzeSynergyScoreRequest struct {
	AnchorHeroID string `json:"anchorHeroId" example:"tigreal"`
	Language     string `json:"language" enums:"en,id" example:"en"`
}

// AnalyzeCounterDetailRequest asks for an AI explanation of one counter matchup.
type AnalyzeCounterDetailRequest struct {
	TargetHeroID  string `json:"targetHeroId" example:"tigreal"`
	CounterHeroID string `json:"counterHeroId" example:"diggie"`
	Language      string `json:"language" enums:"en,id" example:"en"`
}

// AnalyzeSynergyDetailRequest asks for an AI explanation of one synergy pairing.
type AnalyzeSynergyDetailRequest struct {
	AnchorHeroID  string `json:"anchorHeroId" example:"tigreal"`
	SynergyHeroID string `json:"synergyHeroId" example:"pharsa"`
	Language      string `json:"language" enums:"en,id" example:"en"`
}

// CounterScoreRecommendation is one scored counter, ranked best-first.
type CounterScoreRecommendation struct {
	Rank          int    `json:"rank" example:"1"`
	CounterHeroID string `json:"counterHeroId" example:"diggie"`
	Score         int    `json:"score" example:"90"`
	Confidence    int    `json:"confidence" example:"85"`
}

// SynergyScoreRecommendation is one scored synergy partner, ranked best-first.
type SynergyScoreRecommendation struct {
	Rank          int    `json:"rank" example:"1"`
	SynergyHeroID string `json:"synergyHeroId" example:"pharsa"`
	Score         int    `json:"score" example:"90"`
	Confidence    int    `json:"confidence" example:"85"`
}

// AnalyzeCounterScoreResponse ranks every counter of the target hero.
type AnalyzeCounterScoreResponse struct {
	TargetHeroID    string                       `json:"targetHeroId" example:"tigreal"`
	Source          string                       `json:"source" example:"ai"`
	Recommendations []CounterScoreRecommendation `json:"recommendations"`
}

// AnalyzeSynergyScoreResponse ranks every synergy partner of the anchor hero.
type AnalyzeSynergyScoreResponse struct {
	AnchorHeroID    string                       `json:"anchorHeroId" example:"tigreal"`
	Source          string                       `json:"source" example:"ai"`
	Recommendations []SynergyScoreRecommendation `json:"recommendations"`
}

// AnalyzeCounterDetailResponse explains one counter matchup from authored evidence.
type AnalyzeCounterDetailResponse struct {
	TargetHeroID  string   `json:"targetHeroId" example:"tigreal"`
	CounterHeroID string   `json:"counterHeroId" example:"diggie"`
	Source        string   `json:"source" example:"ai"`
	Score         int      `json:"score" example:"90"`
	Confidence    int      `json:"confidence" example:"85"`
	Summary       string   `json:"summary"`
	Strengths     []string `json:"strengths"`
	Conditions    []string `json:"conditions"`
	FailureCases  []string `json:"failureCases"`
	EvidenceIDs   []string `json:"evidenceIds"`
}

// AnalyzeSynergyDetailResponse explains one synergy pairing from authored evidence.
type AnalyzeSynergyDetailResponse struct {
	AnchorHeroID  string   `json:"anchorHeroId" example:"tigreal"`
	SynergyHeroID string   `json:"synergyHeroId" example:"pharsa"`
	Source        string   `json:"source" example:"ai"`
	Score         int      `json:"score" example:"90"`
	Confidence    int      `json:"confidence" example:"85"`
	Summary       string   `json:"summary"`
	Strengths     []string `json:"strengths"`
	Conditions    []string `json:"conditions"`
	FailureCases  []string `json:"failureCases"`
	EvidenceIDs   []string `json:"evidenceIds"`
}

// HealthResponse is the liveness payload.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
