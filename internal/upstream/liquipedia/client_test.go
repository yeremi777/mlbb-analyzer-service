package liquipedia

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func stubServer(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL)), srv
}

func parseEnvelope(inner string) map[string]any {
	return map[string]any{"parse": map[string]any{"text": map[string]any{"*": inner}}}
}

func TestFetchPatchesReturnsParsedCalendar(t *testing.T) {
	var (
		gotUA    string
		gotQuery string
		calls    int
	)
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotUA = r.Header.Get("User-Agent")
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(parseEnvelope(oneTable))
	})

	got, err := client.FetchPatches(context.Background())
	if err != nil {
		t.Fatalf("FetchPatches: %v", err)
	}
	if len(got) != 2 || got[0].Version != "2.1.95" {
		t.Fatalf("unexpected patches: %+v", got)
	}

	// Liquipedia's terms require a descriptive UA with contact details, and cap
	// action=parse at one request per 30s: one call per run keeps us inside it.
	if calls != 1 {
		t.Errorf("want exactly 1 request, got %d", calls)
	}
	if !strings.Contains(gotUA, "mlbb-analyzer") || !strings.Contains(gotUA, "@") {
		t.Errorf("User-Agent lacks project or contact details: %q", gotUA)
	}
	if !strings.Contains(gotQuery, "action=parse") || !strings.Contains(gotQuery, "Portal%3APatches") {
		t.Errorf("unexpected query: %q", gotQuery)
	}
}

func TestFetchPatchesHTTPErrorIsError(t *testing.T) {
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	})
	recordSleeps(client)
	if _, err := client.FetchPatches(context.Background()); err == nil {
		t.Fatal("want an error on non-200")
	}
}

func TestFetchPatchesMediaWikiErrorIsError(t *testing.T) {
	// MediaWiki reports failure in the body with HTTP 200.
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "missingtitle", "info": "page does not exist"}})
	})
	if _, err := client.FetchPatches(context.Background()); err == nil {
		t.Fatal("want an error when MediaWiki returns an error envelope")
	}
}

func TestFetchPatchesEmptyPageIsError(t *testing.T) {
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(parseEnvelope("<p>no table here</p>"))
	})
	recordSleeps(client)
	if _, err := client.FetchPatches(context.Background()); err == nil {
		t.Fatal("want an error when the page carries no patches")
	}
}

func TestFetchPatchesHandlesGzip(t *testing.T) {
	// Liquipedia requires gzip and serves compressed bodies. Go's transport
	// decompresses transparently only when it set Accept-Encoding itself, so a
	// hand-set header hands back raw gzip bytes.
	var sawAcceptEncoding string
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawAcceptEncoding = r.Header.Get("Accept-Encoding")
		if !strings.Contains(sawAcceptEncoding, "gzip") {
			t.Errorf("client did not offer gzip: %q", sawAcceptEncoding)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		defer zw.Close()
		if err := json.NewEncoder(zw).Encode(parseEnvelope(oneTable)); err != nil {
			t.Fatal(err)
		}
	})

	got, err := client.FetchPatches(context.Background())
	if err != nil {
		t.Fatalf("FetchPatches on a gzipped body: %v", err)
	}
	if len(got) != 2 || got[0].Version != "2.1.95" {
		t.Fatalf("unexpected patches: %+v", got)
	}
}

// recordSleeps captures backoff instead of waiting.
func recordSleeps(c *Client) *[]time.Duration {
	var got []time.Duration
	c.sleep = func(d time.Duration) { got = append(got, d) }
	return &got
}

func TestFetchPatchesRetriesThrottleThenSucceeds(t *testing.T) {
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(parseEnvelope(oneTable))
	})
	slept := recordSleeps(client)

	got, err := client.FetchPatches(context.Background())
	if err != nil {
		t.Fatalf("FetchPatches: %v", err)
	}
	if len(got) != 2 || calls != 2 {
		t.Fatalf("want 2 patches after 2 calls, got %d patches in %d calls", len(got), calls)
	}
	// Liquipedia caps action=parse at one request per 30s; backing off less
	// than that would compound the throttle we just hit.
	if len(*slept) != 1 || (*slept)[0] < 30*time.Second {
		t.Errorf("want a backoff of at least 30s, got %v", *slept)
	}
}

func TestFetchPatchesHonoursRetryAfter(t *testing.T) {
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "90")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(parseEnvelope(oneTable))
	})
	slept := recordSleeps(client)

	if _, err := client.FetchPatches(context.Background()); err != nil {
		t.Fatalf("FetchPatches: %v", err)
	}
	if len(*slept) != 1 || (*slept)[0] != 90*time.Second {
		t.Errorf("want the server's Retry-After of 90s, got %v", *slept)
	}
}

func TestFetchPatchesRetriesOddBodyThenSucceeds(t *testing.T) {
	// The observed live failure: HTTP 200 carrying a body with no tables.
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(parseEnvelope("<p>Too many requests</p>"))
			return
		}
		_ = json.NewEncoder(w).Encode(parseEnvelope(oneTable))
	})
	recordSleeps(client)

	got, err := client.FetchPatches(context.Background())
	if err != nil {
		t.Fatalf("FetchPatches: %v", err)
	}
	if len(got) != 2 || calls != 2 {
		t.Fatalf("want recovery on the second call, got %d patches in %d calls", len(got), calls)
	}
}

func TestFetchPatchesDoesNotRetryStructureChange(t *testing.T) {
	// A redesigned page parses the same way every time: retrying only doubles
	// the load on an upstream that is behaving correctly.
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(parseEnvelope(
			`<table class="wikitable"><tbody><tr><td>Fanny</td><td>Assassin</td></tr></tbody></table>`))
	})
	recordSleeps(client)

	_, err := client.FetchPatches(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if calls != 1 {
		t.Errorf("structure change must not be retried, got %d calls", calls)
	}
	if !strings.Contains(err.Error(), "structure changed") {
		t.Errorf("want a structure-change diagnosis, got %q", err)
	}
}

func TestFetchPatchesDoesNotRetryClientError(t *testing.T) {
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "nope", http.StatusNotFound)
	})
	recordSleeps(client)

	if _, err := client.FetchPatches(context.Background()); err == nil {
		t.Fatal("want an error")
	}
	if calls != 1 {
		t.Errorf("404 must not be retried, got %d calls", calls)
	}
}

func TestFetchPatchesExhaustsRetriesAndReportsAttempts(t *testing.T) {
	calls := 0
	client, _ := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "slow down", http.StatusTooManyRequests)
	})
	recordSleeps(client)

	_, err := client.FetchPatches(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if calls != 2 {
		t.Errorf("want 2 attempts, got %d", calls)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should name the upstream status: %q", err)
	}
}
