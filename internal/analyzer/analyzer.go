// Package analyzer turns a matchup into a score or a written explanation by
// prompting an AI provider, with a cache in front and a fallback chain behind.
// It is called by api and speaks only in domain types.
package analyzer

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

// Error is a structured analyzer failure carrying the API error code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func providerFailure(format string, args ...any) *Error {
	return &Error{Code: "ai_provider_error", Message: fmt.Sprintf(format, args...)}
}

type Config struct {
	CacheTTL        time.Duration
	CacheMaxEntries int
}

type Analyzer struct {
	provider Provider
	cfg      Config

	mu    sync.Mutex
	cache map[string]*list.Element
	lru   *list.List
}

type cacheEntry struct {
	key       string
	expiresAt time.Time
	value     any
}

func New(provider Provider, cfg Config) *Analyzer {
	return &Analyzer{provider: provider, cfg: cfg, cache: map[string]*list.Element{}, lru: list.New()}
}

func (a *Analyzer) cacheGet(key string) (any, bool) {
	if a.cfg.CacheTTL <= 0 || a.cfg.CacheMaxEntries <= 0 {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	el, ok := a.cache[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		a.lru.Remove(el)
		delete(a.cache, key)
		return nil, false
	}
	a.lru.MoveToBack(el)
	return entry.value, true
}

func (a *Analyzer) cacheSet(key string, value any) {
	if a.cfg.CacheTTL <= 0 || a.cfg.CacheMaxEntries <= 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if el, ok := a.cache[key]; ok {
		a.lru.Remove(el)
		delete(a.cache, key)
	}
	el := a.lru.PushBack(&cacheEntry{key: key, expiresAt: time.Now().Add(a.cfg.CacheTTL), value: value})
	a.cache[key] = el
	for len(a.cache) > a.cfg.CacheMaxEntries {
		oldest := a.lru.Front()
		a.lru.Remove(oldest)
		delete(a.cache, oldest.Value.(*cacheEntry).key)
	}
}

// HasCachedScore reports whether a scoring result is cached; cached responses
// skip the rate limiter.
func (a *Analyzer) HasCachedScore(kind, heroID, language string) bool {
	_, ok := a.cacheGet(kind + "-score:" + heroID + ":" + language)
	return ok
}

// HasCachedDetail is HasCachedScore for one matchup's detail analysis.
func (a *Analyzer) HasCachedDetail(kind, heroID, partnerID, language string) bool {
	_, ok := a.cacheGet(kind + "-detail:" + heroID + ":" + partnerID + ":" + language)
	return ok
}

type ScoreRecommendation struct {
	Rank          int    `json:"rank"`
	CounterHeroID string `json:"counterHeroId"`
	Score         int    `json:"score"`
	Confidence    int    `json:"confidence"`
}

type ScoresResult struct {
	Recommendations []ScoreRecommendation
}

type DetailResult struct {
	Score        int
	Confidence   int
	Summary      string
	Strengths    []string
	Conditions   []string
	FailureCases []string
	EvidenceIDs  []string
}

func (a *Analyzer) complete(ctx context.Context, messages []Message) (map[string]any, error) {
	payload, err := a.provider.CompleteJSON(ctx, messages)
	if err != nil {
		if pe, ok := err.(*ProviderError); ok {
			return nil, providerFailure("%s", pe.Message)
		}
		if ae, ok := err.(*Error); ok {
			return nil, ae
		}
		return nil, providerFailure("%v", err)
	}
	return payload, nil
}

type scoringItem struct {
	HeroID     string
	Score      int
	Confidence int
}

func validateScoring(payload map[string]any, idKey string, expected map[string]bool) ([]scoringItem, error) {
	raw, ok := payload["recommendations"].([]any)
	if !ok {
		return nil, providerFailure("Invalid scoring response from model: recommendations must be an array")
	}
	items := make([]scoringItem, 0, len(raw))
	seen := map[string]bool{}
	for _, entry := range raw {
		obj, ok := entry.(map[string]any)
		if !ok {
			return nil, providerFailure("Invalid scoring response from model: recommendation must be an object")
		}
		id, _ := obj[idKey].(string)
		score, okS := intField(obj, "score")
		confidence, okC := intField(obj, "confidence")
		if id == "" || !okS || !okC {
			return nil, providerFailure("Invalid scoring response from model: missing %s, score, or confidence", idKey)
		}
		seen[id] = true
		items = append(items, scoringItem{HeroID: id, Score: score, Confidence: confidence})
	}
	if len(items) != len(expected) || len(seen) != len(expected) {
		return nil, providerFailure("Scoring response must include every expected %s once.", idKey)
	}
	for id := range expected {
		if !seen[id] {
			return nil, providerFailure("Scoring response must include every expected %s once.", idKey)
		}
	}
	return items, nil
}

func intField(obj map[string]any, key string) (int, bool) {
	f, ok := obj[key].(float64)
	if !ok || f < 0 || f > 100 || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func rank(items []scoringItem) []ScoreRecommendation {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].Confidence != items[j].Confidence {
			return items[i].Confidence > items[j].Confidence
		}
		return items[i].HeroID < items[j].HeroID
	})
	out := make([]ScoreRecommendation, len(items))
	for i, item := range items {
		out[i] = ScoreRecommendation{Rank: i + 1, CounterHeroID: item.HeroID, Score: item.Score, Confidence: item.Confidence}
	}
	return out
}

func (a *Analyzer) score(ctx context.Context, cacheKey, idKey string, messages []Message, ms []domain.HeroMatchup) (*ScoresResult, error) {
	if cached, ok := a.cacheGet(cacheKey); ok {
		return cached.(*ScoresResult), nil
	}
	expected := map[string]bool{}
	for _, m := range ms {
		expected[m.Second.UID] = true
	}
	payload, err := a.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	items, err := validateScoring(payload, idKey, expected)
	if err != nil {
		return nil, err
	}
	result := &ScoresResult{Recommendations: rank(items)}
	a.cacheSet(cacheKey, result)
	return result, nil
}

// ScoreCounters scores every counter matchup of the target hero in one call.
func (a *Analyzer) ScoreCounters(ctx context.Context, target domain.Hero, ms []domain.HeroMatchup, language string) (*ScoresResult, error) {
	return a.score(ctx, "counter-score:"+target.UID+":"+language, "counterHeroId",
		buildScoringMessages(target, ms, language), ms)
}

// ScoreSynergies scores every synergy pairing of the anchor hero in one call.
func (a *Analyzer) ScoreSynergies(ctx context.Context, anchor domain.Hero, ms []domain.HeroMatchup, language string) (*ScoresResult, error) {
	return a.score(ctx, "synergy-score:"+anchor.UID+":"+language, "synergyHeroId",
		buildSynergyScoringMessages(anchor, ms, language), ms)
}

func validateDetail(payload map[string]any, allowedIDs map[string]bool) (*DetailResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, providerFailure("%v", err)
	}
	var parsed struct {
		Score        *float64 `json:"score"`
		Confidence   *float64 `json:"confidence"`
		Summary      string   `json:"summary"`
		Strengths    []string `json:"strengths"`
		Conditions   []string `json:"conditions"`
		FailureCases []string `json:"failureCases"`
		EvidenceIDs  []string `json:"evidenceIds"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("detail payload has wrong field types: %w", err)
	}
	if parsed.Score == nil || *parsed.Score < 0 || *parsed.Score > 100 ||
		parsed.Confidence == nil || *parsed.Confidence < 0 || *parsed.Confidence > 100 ||
		parsed.Summary == "" || len(parsed.Strengths) == 0 {
		return nil, fmt.Errorf("detail payload missing required fields: score, confidence, non-empty summary, non-empty strengths")
	}
	for _, id := range parsed.EvidenceIDs {
		if !allowedIDs[id] {
			return nil, providerFailure("Detail response referenced unknown evidence ids.")
		}
	}
	return &DetailResult{
		Score: int(*parsed.Score), Confidence: int(*parsed.Confidence), Summary: parsed.Summary,
		Strengths: parsed.Strengths, Conditions: orEmpty(parsed.Conditions),
		FailureCases: orEmpty(parsed.FailureCases), EvidenceIDs: orEmpty(parsed.EvidenceIDs),
	}, nil
}

func (a *Analyzer) detail(ctx context.Context, cacheKey string, messages []Message, m domain.HeroMatchup, language string) (*DetailResult, error) {
	if cached, ok := a.cacheGet(cacheKey); ok {
		return cached.(*DetailResult), nil
	}
	allowed := map[string]bool{}
	for _, p := range m.Proof {
		allowed[p.ID] = true
	}

	payload, err := a.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	result, err := validateDetail(payload, allowed)
	if err != nil {
		if _, fatal := err.(*Error); fatal {
			return nil, err
		}
		repair := buildDetailRepairMessages(messages, payload, err, language)
		retryPayload, retryErr := a.complete(ctx, repair)
		if retryErr != nil {
			return nil, retryErr
		}
		result, err = validateDetail(retryPayload, allowed)
		if err != nil {
			if ae, ok := err.(*Error); ok {
				return nil, ae
			}
			return nil, providerFailure("Invalid detail response from model after retry: %v", err)
		}
	}
	a.cacheSet(cacheKey, result)
	return result, nil
}

// CounterDetail explains one counter matchup, retrying once with a repair
// prompt when the model's first JSON fails validation.
func (a *Analyzer) CounterDetail(ctx context.Context, target domain.Hero, m domain.HeroMatchup, language string) (*DetailResult, error) {
	return a.detail(ctx, "counter-detail:"+target.UID+":"+m.Second.UID+":"+language,
		buildDetailMessages(target, m, language), m, language)
}

// SynergyDetail is CounterDetail for one synergy pairing.
func (a *Analyzer) SynergyDetail(ctx context.Context, anchor domain.Hero, m domain.HeroMatchup, language string) (*DetailResult, error) {
	return a.detail(ctx, "synergy-detail:"+anchor.UID+":"+m.Second.UID+":"+language,
		buildSynergyDetailMessages(anchor, m, language), m, language)
}
