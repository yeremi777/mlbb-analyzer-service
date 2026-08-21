package ratelimit

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testLimiter(t *testing.T, maxRequests int) *Limiter {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("local redis unavailable: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return New(client, Config{
		Enabled:          true,
		MaxRequests:      maxRequests,
		WindowSeconds:    60,
		DetailMultiplier: 3,
		CookieName:       "mlbb_analyzer_client_id",
		CookieMaxAge:     3600,
		Salt:             "test-salt",
	})
}

// unique endpoint name per test run so counters never collide across runs
func endpoint(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
}

func TestAllowsUpToLimitThen429(t *testing.T) {
	l := testLimiter(t, 2)
	ep := endpoint("score")
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/x", nil)
		if err := l.Enforce(w, r, ep); err != nil {
			t.Fatalf("request %d should pass: %v", i+1, err)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	err := l.Enforce(w, r, ep)
	if err == nil {
		t.Fatal("third request should be limited")
	}
	le, ok := err.(*Error)
	if !ok || le.Status != 429 || le.Code != "rate_limit_exceeded" || le.RetryAfter <= 0 {
		t.Fatalf("wrong limit error: %+v", err)
	}
}

func TestSetsClientCookieOnFirstRequest(t *testing.T) {
	l := testLimiter(t, 5)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	if err := l.Enforce(w, r, endpoint("cookie")); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "mlbb_analyzer_client_id" || !cookies[0].HttpOnly {
		t.Fatalf("expected one httponly client cookie, got %+v", cookies)
	}
}

func TestDetailEndpointGetsMultiplier(t *testing.T) {
	l := testLimiter(t, 1)
	ep := endpoint("analyze-counter") + "-detail" // suffix rule from the contract
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/x", nil)
		if err := l.Enforce(w, r, ep); err != nil {
			t.Fatalf("detail request %d should pass (1x3 budget): %v", i+1, err)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	if l.Enforce(w, r, ep) == nil {
		t.Fatal("fourth detail request should be limited")
	}
}

func TestDisabledLimiterAllowsEverything(t *testing.T) {
	l := testLimiter(t, 1)
	l.cfg.Enabled = false
	ep := endpoint("disabled")
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/x", nil)
		if err := l.Enforce(w, r, ep); err != nil {
			t.Fatal(err)
		}
	}
}

func TestForwardedForTakesFirstIP(t *testing.T) {
	l := testLimiter(t, 1)
	ep := endpoint("xff")
	// two different clients via x-forwarded-for must not share a counter
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/x", nil)
		r.Header.Set("X-Forwarded-For", ip+", 10.0.0.1")
		if err := l.Enforce(w, r, ep); err != nil {
			t.Fatalf("ip %s should have its own budget: %v", ip, err)
		}
	}
}
