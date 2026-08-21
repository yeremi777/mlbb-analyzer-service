package moonton

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode stub response: %v", err)
	}
}

func stubClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL)), srv
}

func TestFetchPopulatesRecordsAndRaw(t *testing.T) {
	client, _ := stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.Header.Get("Origin") != "https://www.mobilelegends.com" {
			t.Errorf("missing Origin header")
		}
		writeJSON(t, w, map[string]any{
			"code": 0, "message": "OK",
			"data": map[string]any{
				"total": 2,
				"records": []map[string]any{
					{"data": map[string]any{
						"main_heroid": 60, "main_hero_win_rate": 0.528754,
						"main_hero_appearance_rate": 0.034504, "main_hero_ban_rate": 0.110542,
						"main_hero": map[string]any{"data": map[string]any{"name": "Hanabi"}},
						"sub_hero":  []map[string]any{{"heroid": 94, "increase_win_rate": 0.02495}}}},
					{"data": map[string]any{
						"main_heroid": 132, "main_hero_win_rate": 0.589272,
						"main_hero_appearance_rate": 0.001679, "main_hero_ban_rate": 0.11355,
						"main_hero": map[string]any{"data": map[string]any{"name": "Marcel"}}}},
				},
			},
		})
	})

	combo := Combo{WindowDays: 1, RankTier: "mythic", EndpointID: 2756567, BigRank: "7"}
	resp, err := client.Fetch(context.Background(), combo)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(resp.Records) != 2 {
		t.Fatalf("want 2 records, got %d", len(resp.Records))
	}
	// Typed fields decoded.
	if resp.Records[0].MainHeroID != 60 || resp.Records[0].WinRate != 0.528754 {
		t.Errorf("record 0 decoded wrong: %+v", resp.Records[0])
	}
	// Raw payload preserved verbatim (must include sub_hero, under data).
	var raw map[string]any
	if err := json.Unmarshal(resp.Records[0].Raw, &raw); err != nil {
		t.Fatalf("raw is not valid JSON: %v", err)
	}
	inner, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("raw missing data wrapper: %s", resp.Records[0].Raw)
	}
	if _, ok := inner["sub_hero"]; !ok {
		t.Errorf("raw payload missing sub_hero: %s", resp.Records[0].Raw)
	}
}

func TestFetchSilentEmptyIsError(t *testing.T) {
	// The dangerous upstream case: HTTP 200, code 0, records null, total 0.
	client, _ := stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"code": 0, "message": "OK", "data": map[string]any{"records": nil, "total": 0}})
	})

	combo := Combo{WindowDays: 7, RankTier: "all", EndpointID: 2756569, BigRank: "101"}
	_, err := client.Fetch(context.Background(), combo)
	if err == nil {
		t.Fatal("expected error for silent empty result, got nil")
	}
	if !IsEmpty(err) {
		t.Fatalf("expected EmptyError, got %T: %v", err, err)
	}
}

func TestFetchNonZeroCodeIsError(t *testing.T) {
	client, _ := stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"code": 1, "message": "boom", "total": 0})
	})
	combo := Combo{WindowDays: 3, RankTier: "epic", EndpointID: 2756568, BigRank: "5"}
	_, err := client.Fetch(context.Background(), combo)
	if err == nil || !IsEmpty(err) {
		t.Fatalf("expected EmptyError for non-zero code, got %v", err)
	}
}

func TestFetchHTTPError(t *testing.T) {
	client, _ := stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	combo := Combo{WindowDays: 1, RankTier: "all", EndpointID: 2756567, BigRank: "101"}
	_, err := client.Fetch(context.Background(), combo)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if IsEmpty(err) {
		t.Errorf("500 should not be EmptyError, got %v", err)
	}
}

func TestAllCombosCount(t *testing.T) {
	combos := AllCombos()
	if len(combos) != 30 {
		t.Fatalf("want 30 combos, got %d", len(combos))
	}
	// Spot-check the mapping.
	byKey := map[string]Combo{}
	for _, c := range combos {
		byKey[c.Key()] = c
	}
	if c := byKey["1d/mythic"]; c.EndpointID != 2756567 || c.BigRank != "7" {
		t.Errorf("1d/mythic combo wrong: %+v", c)
	}
	if c := byKey["30d/glory"]; c.EndpointID != 2756570 || c.BigRank != "9" {
		t.Errorf("30d/glory combo wrong: %+v", c)
	}
}

func TestFetchMissingEndpointID(t *testing.T) {
	client := New(WithBaseURL("http://unused.invalid"))
	combo := Combo{WindowDays: 7, RankTier: "all", BigRank: "101"} // no EndpointID
	_, err := client.Fetch(context.Background(), combo)
	if err == nil {
		t.Fatal("expected error for missing endpoint id")
	}
}

func TestFetchTruncatedResponseIsError(t *testing.T) {
	// Upstream claims 133 heroes but sends 2. Silently storing the short set
	// would look like a clean run and quietly lose 131 heroes.
	client, _ := stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"code": 0, "message": "OK",
			"data": map[string]any{
				"total": 133,
				"records": []map[string]any{
					{"data": map[string]any{"main_heroid": 60}},
					{"data": map[string]any{"main_heroid": 132}},
				},
			},
		})
	})

	combo := Combo{WindowDays: 1, RankTier: "all", EndpointID: 2756567, BigRank: "101"}
	_, err := client.Fetch(context.Background(), combo)
	if err == nil {
		t.Fatal("expected error for truncated response, got nil")
	}
	// Truncation is transient, not a contract change: it must stay retryable,
	// unlike the silent-empty case.
	if IsEmpty(err) {
		t.Errorf("truncation must not be EmptyError (would skip retry), got %v", err)
	}
}
