package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yeremi777/mlbb-analyzer-service/internal/analyzer"
	"github.com/yeremi777/mlbb-analyzer-service/internal/ratelimit"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
	"github.com/yeremi777/mlbb-analyzer-service/internal/store"
)

type fakeAnalyzer struct {
	fail   *analyzer.Error
	cached bool
}

func (f *fakeAnalyzer) ScoreCounters(_ context.Context, _ staticdata.Hero, ms []store.HeroMatchup, _ string) (*analyzer.ScoresResult, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	recs := make([]analyzer.ScoreRecommendation, len(ms))
	for i, m := range ms {
		recs[i] = analyzer.ScoreRecommendation{Rank: i + 1, CounterHeroID: m.Second.UID, Score: 90 - i, Confidence: 80}
	}
	return &analyzer.ScoresResult{Recommendations: recs}, nil
}

func (f *fakeAnalyzer) ScoreSynergies(ctx context.Context, h staticdata.Hero, ms []store.HeroMatchup, l string) (*analyzer.ScoresResult, error) {
	return f.ScoreCounters(ctx, h, ms, l)
}

func (f *fakeAnalyzer) CounterDetail(_ context.Context, _ staticdata.Hero, m store.HeroMatchup, _ string) (*analyzer.DetailResult, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	return &analyzer.DetailResult{Score: 88, Confidence: 75, Summary: "sum", Strengths: []string{"s"},
		Conditions: []string{}, FailureCases: []string{}, EvidenceIDs: []string{m.Proof[0].ID}}, nil
}

func (f *fakeAnalyzer) SynergyDetail(ctx context.Context, h staticdata.Hero, m store.HeroMatchup, l string) (*analyzer.DetailResult, error) {
	return f.CounterDetail(ctx, h, m, l)
}

func (f *fakeAnalyzer) HasCachedScore(_, _, _ string) bool     { return f.cached }
func (f *fakeAnalyzer) HasCachedDetail(_, _, _, _ string) bool { return f.cached }

func analyzeServer(t *testing.T, ai Analyzer) *httptest.Server {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:root@127.0.0.1:5432/mlbb_analyzer")
	if err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	srv := httptest.NewServer(NewServer(pool, ai, nil).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func postJSON(t *testing.T, url, body string, dst any) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.StatusCode
}

func TestAnalyzeScore(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{})
	var res struct {
		TargetHeroID    string `json:"targetHeroId"`
		Source          string `json:"source"`
		Recommendations []struct {
			Rank          int    `json:"rank"`
			CounterHeroID string `json:"counterHeroId"`
			Score         int    `json:"score"`
		} `json:"recommendations"`
	}
	code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, &res)
	if code != 200 || res.Source != "ai" || res.TargetHeroID != "tigreal" || len(res.Recommendations) != 5 {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestAnalyzeScoreUnknownHero(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{})
	var res struct {
		Error struct{ Code string } `json:"error"`
	}
	code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"nope"}`, &res)
	if code != 404 || res.Error.Code != "target_hero_not_found" {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestAnalyzeDetail(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{})
	var res struct {
		TargetHeroID  string   `json:"targetHeroId"`
		CounterHeroID string   `json:"counterHeroId"`
		Source        string   `json:"source"`
		Score         int      `json:"score"`
		Strengths     []string `json:"strengths"`
		Conditions    []string `json:"conditions"`
	}
	code := postJSON(t, srv.URL+"/api/counters/analyze-detail", `{"targetHeroId":"tigreal","counterHeroId":"diggie"}`, &res)
	if code != 200 || res.Score != 88 || res.CounterHeroID != "diggie" || res.Conditions == nil {
		t.Fatalf("code=%d res=%+v (conditions must be [], not null)", code, res)
	}
}

func TestAnalyzeDetailUnknownMatchup(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{})
	var res struct {
		Error struct{ Code string } `json:"error"`
	}
	code := postJSON(t, srv.URL+"/api/counters/analyze-detail", `{"targetHeroId":"tigreal","counterHeroId":"miya"}`, &res)
	if code != 404 || res.Error.Code != "counter_matchup_not_found" {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestAnalyzeProviderErrorMapsTo502(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{fail: &analyzer.Error{Code: "ai_provider_error", Message: "boom"}})
	var res struct {
		Error struct{ Code string } `json:"error"`
	}
	code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, &res)
	if code != 502 || res.Error.Code != "ai_provider_error" {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestAnalyzeWithoutAnalyzer504(t *testing.T) {
	srv := analyzeServer(t, nil)
	var res struct {
		Error struct{ Code string } `json:"error"`
	}
	code := postJSON(t, srv.URL+"/api/synergies/analyze-score", `{"anchorHeroId":"tigreal"}`, &res)
	if code != 504 || res.Error.Code != "ai_provider_not_configured" {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestSynergyAnalyzeScore(t *testing.T) {
	srv := analyzeServer(t, &fakeAnalyzer{})
	var res struct {
		AnchorHeroID    string `json:"anchorHeroId"`
		Recommendations []struct {
			SynergyHeroID string `json:"synergyHeroId"`
		} `json:"recommendations"`
	}
	code := postJSON(t, srv.URL+"/api/synergies/analyze-score", `{"anchorHeroId":"tigreal"}`, &res)
	if code != 200 || res.AnchorHeroID != "tigreal" || len(res.Recommendations) == 0 || res.Recommendations[0].SynergyHeroID == "" {
		t.Fatalf("code=%d res=%+v", code, res)
	}
}

func TestAnalyzeRateLimit(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:root@127.0.0.1:5432/mlbb_analyzer")
	if err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("local redis unavailable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	limiter := ratelimit.New(rdb, ratelimit.Config{
		Enabled: true, MaxRequests: 1, WindowSeconds: 60, DetailMultiplier: 3,
		CookieName: "mlbb_analyzer_client_id", CookieMaxAge: 3600,
		Salt: fmt.Sprintf("test-%d", time.Now().UnixNano()), // unique salt = fresh ip counter per run
	})
	fake := &fakeAnalyzer{}
	srv := httptest.NewServer(NewServer(pool, fake, limiter).Handler())
	t.Cleanup(srv.Close)

	var ok map[string]any
	if code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, &ok); code != 200 {
		t.Fatalf("first request: %d", code)
	}
	// same ip, no cookie carried: second request must hit the ip counter
	var errBody struct {
		Error struct{ Code string } `json:"error"`
	}
	if code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, &errBody); code != 429 || errBody.Error.Code != "rate_limit_exceeded" {
		t.Fatalf("second request: code=%d body=%+v", code, errBody)
	}
	// cached analyses bypass the limiter entirely
	fake.cached = true
	if code := postJSON(t, srv.URL+"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, &ok); code != 200 {
		t.Fatalf("cached request should bypass limiter: %d", code)
	}
}
