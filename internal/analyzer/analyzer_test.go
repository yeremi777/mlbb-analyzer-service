package analyzer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

type fakeProvider struct {
	responses []map[string]any
	errs      []error
	calls     int
}

func (f *fakeProvider) Name() string { return "Fake" }

func (f *fakeProvider) CompleteJSON(_ context.Context, _ []Message) (map[string]any, error) {
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return nil, errors.New("fake exhausted")
}

func hero(uid string) domain.Hero {
	return domain.Hero{UID: uid, MLID: 1, Name: uid, Roles: []string{"tank"}}
}

func matchups() []domain.HeroMatchup {
	return []domain.HeroMatchup{
		{First: "tigreal", Second: hero("diggie"), Reasons: []string{"r"}, Types: []string{"anti-cc"},
			Proof: []domain.Proof{{ID: "p1", Category: "skill-interaction", Priority: "primary", Impact: "high", Summary: "s"}}},
		{First: "tigreal", Second: hero("valir"), Reasons: []string{"r"}, Types: []string{"burst"},
			Proof: []domain.Proof{{ID: "p2", Category: "skill-interaction", Priority: "primary", Impact: "high", Summary: "s"}}},
	}
}

func newTestAnalyzer(p Provider) *Analyzer {
	return New(p, Config{CacheTTL: time.Minute, CacheMaxEntries: 16})
}

func TestScoreCountersRanksAndCaches(t *testing.T) {
	fake := &fakeProvider{responses: []map[string]any{{
		"recommendations": []any{
			map[string]any{"counterHeroId": "diggie", "score": 80.0, "confidence": 70.0},
			map[string]any{"counterHeroId": "valir", "score": 90.0, "confidence": 60.0},
		},
	}}}
	a := newTestAnalyzer(fake)

	res, err := a.ScoreCounters(context.Background(), hero("tigreal"), matchups(), "en")
	if err != nil {
		t.Fatal(err)
	}
	if res.Recommendations[0].CounterHeroID != "valir" || res.Recommendations[0].Rank != 1 {
		t.Fatalf("ranking wrong: %+v", res.Recommendations)
	}
	if res.Recommendations[1].Rank != 2 {
		t.Fatalf("rank 2 wrong: %+v", res.Recommendations[1])
	}

	// second call must hit the cache, not the provider
	if _, err := a.ScoreCounters(context.Background(), hero("tigreal"), matchups(), "en"); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("provider called %d times, want 1 (cache miss)", fake.calls)
	}
}

func TestScoreCountersRejectsMissingHero(t *testing.T) {
	fake := &fakeProvider{responses: []map[string]any{{
		"recommendations": []any{
			map[string]any{"counterHeroId": "diggie", "score": 80.0, "confidence": 70.0},
		},
	}}}
	a := newTestAnalyzer(fake)
	_, err := a.ScoreCounters(context.Background(), hero("tigreal"), matchups(), "en")
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != "ai_provider_error" {
		t.Fatalf("want ai_provider_error, got %v", err)
	}
}

func TestDetailRepairRetry(t *testing.T) {
	bad := map[string]any{"score": 80.0} // missing required keys
	good := map[string]any{
		"score": 80.0, "confidence": 70.0, "summary": "sum",
		"strengths": []any{"s1"}, "conditions": []any{}, "failureCases": []any{},
		"evidenceIds": []any{"p1"},
	}
	fake := &fakeProvider{responses: []map[string]any{bad, good}}
	a := newTestAnalyzer(fake)

	ms := matchups()
	res, err := a.CounterDetail(context.Background(), hero("tigreal"), ms[0], "en")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 {
		t.Fatalf("provider called %d times, want 2 (repair retry)", fake.calls)
	}
	if res.Score != 80 || res.Summary != "sum" || len(res.Strengths) != 1 {
		t.Fatalf("bad response %+v", res)
	}
}

func TestDetailRejectsUnknownEvidence(t *testing.T) {
	payload := map[string]any{
		"score": 80.0, "confidence": 70.0, "summary": "sum",
		"strengths": []any{"s1"}, "evidenceIds": []any{"unknown-id"},
	}
	fake := &fakeProvider{responses: []map[string]any{payload, payload}}
	a := newTestAnalyzer(fake)
	ms := matchups()
	if _, err := a.CounterDetail(context.Background(), hero("tigreal"), ms[0], "en"); err == nil {
		t.Fatal("want error for unknown evidence id")
	}
}

func TestProviderErrorMapping(t *testing.T) {
	fake := &fakeProvider{errs: []error{&ProviderError{Message: "OpenRouter returned HTTP 500", Retryable: true}}}
	a := newTestAnalyzer(fake)
	_, err := a.ScoreCounters(context.Background(), hero("tigreal"), matchups(), "en")
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != "ai_provider_error" {
		t.Fatalf("want ai_provider_error, got %v", err)
	}
}
