package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:root@127.0.0.1:5432/mlbb_analyzer")
	if err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	srv := httptest.NewServer(NewServer(pool, nil, nil).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string, dst any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return resp.StatusCode
}

func TestHealth(t *testing.T) {
	srv := testServer(t)
	var body map[string]string
	if code := getJSON(t, srv.URL+"/health", &body); code != 200 {
		t.Fatalf("status %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("got %v", body)
	}
}

func TestListHeroesPaginationAndFilters(t *testing.T) {
	srv := testServer(t)
	var page struct {
		Items []struct {
			UID  string `json:"uid"`
			MLID string `json:"mlid"`
		} `json:"items"`
		Page, Size, Total, Pages int
	}
	if code := getJSON(t, srv.URL+"/api/heroes?page=1&size=10", &page); code != 200 {
		t.Fatalf("status %d", code)
	}
	if page.Total != 132 || page.Pages != 14 || len(page.Items) != 10 {
		t.Fatalf("total=%d pages=%d items=%d", page.Total, page.Pages, len(page.Items))
	}
	if page.Items[0].UID != "miya" || page.Items[0].MLID != "1" {
		t.Fatalf("first item %+v (mlid must serialize as string)", page.Items[0])
	}

	if code := getJSON(t, srv.URL+"/api/heroes?role=TANK&size=100", &page); code != 200 {
		t.Fatalf("status %d", code)
	}
	if page.Total == 0 || page.Total > 40 {
		t.Fatalf("role filter total=%d", page.Total)
	}
	if code := getJSON(t, srv.URL+"/api/heroes?search=miy", &page); code != 200 || page.Total != 1 {
		t.Fatalf("search: code=%d total=%d", code, page.Total)
	}
}

func TestGetHeroAndNotFound(t *testing.T) {
	srv := testServer(t)
	var hero struct {
		UID    string `json:"uid"`
		Images struct {
			Head string `json:"head"`
		} `json:"images"`
	}
	if code := getJSON(t, srv.URL+"/api/heroes/tigreal", &hero); code != 200 || hero.UID != "tigreal" {
		t.Fatalf("code=%d hero=%+v", code, hero)
	}
	var errBody struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if code := getJSON(t, srv.URL+"/api/heroes/nonexistent", &errBody); code != 404 {
		t.Fatalf("status %d", code)
	}
	if errBody.Error.Code != "hero_not_found" {
		t.Fatalf("got code %q", errBody.Error.Code)
	}
}

func TestListHeroCounters(t *testing.T) {
	srv := testServer(t)
	var ms []struct {
		TargetHeroID string `json:"targetHeroId"`
		CounterHero  struct {
			UID string `json:"uid"`
		} `json:"counterHero"`
		Reasons      []string `json:"reasons"`
		CounterTypes []string `json:"counterTypes"`
		Proof        []struct {
			ID            string   `json:"id"`
			Category      string   `json:"category"`
			WorksBestWhen []string `json:"worksBestWhen"`
		} `json:"proof"`
	}
	if code := getJSON(t, srv.URL+"/api/heroes/tigreal/counters", &ms); code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(ms) != 5 {
		t.Fatalf("got %d matchups, want 5", len(ms))
	}
	m := ms[0]
	if m.TargetHeroID != "tigreal" || m.CounterHero.UID == "" || len(m.Reasons) == 0 || len(m.Proof) == 0 {
		t.Fatalf("bad matchup %+v", m)
	}
	if m.Proof[0].WorksBestWhen == nil {
		t.Fatal("worksBestWhen must serialize as [], not null")
	}

	var errBody struct {
		Error struct{ Code string } `json:"error"`
	}
	if code := getJSON(t, srv.URL+"/api/heroes/nonexistent/counters", &errBody); code != 404 || errBody.Error.Code != "hero_not_found" {
		t.Fatalf("code=%d body=%+v", code, errBody)
	}
}

func TestListHeroSynergies(t *testing.T) {
	srv := testServer(t)
	var ms []struct {
		AnchorHeroID string `json:"anchorHeroId"`
		SynergyHero  struct {
			UID string `json:"uid"`
		} `json:"synergyHero"`
	}
	if code := getJSON(t, srv.URL+"/api/heroes/tigreal/synergies", &ms); code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(ms) == 0 || ms[0].AnchorHeroID != "tigreal" || ms[0].SynergyHero.UID == "" {
		t.Fatalf("bad synergies %+v", ms)
	}
}
