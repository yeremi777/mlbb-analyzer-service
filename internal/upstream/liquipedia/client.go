package liquipedia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

const (
	defaultBaseURL = "https://liquipedia.net/mobilelegends/api.php"
	patchesPage    = "Portal:Patches"
	defaultUA      = "mlbb-analyzer-collector/1.0 (+github.com/yeremi777/mlbb-analyzer-service; kuroganehunter99@gmail.com)"

	// Liquipedia caps action=parse at one request per 30 seconds. A retry that
	// waited less than that would compound the throttle it is recovering from.
	defaultAttempts = 2
	defaultBackoff  = 30 * time.Second
)

// Client reads the patch calendar from Liquipedia's MediaWiki API.
//
// Liquipedia's API terms cap action=parse at one request per 30 seconds and
// require a descriptive User-Agent carrying contact details. One request per
// run satisfies both; the terms also forbid scraping the rendered page, so
// this always goes through api.php.
type Client struct {
	baseURL   string
	http      *http.Client
	userAgent string
	attempts  int
	backoff   time.Duration
	sleep     func(time.Duration)
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the MediaWiki endpoint (for tests/stubs).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient overrides the HTTP client (for tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithUserAgent overrides the polite User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// New creates a Client pointed at the live Liquipedia API.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:   defaultBaseURL,
		http:      &http.Client{Timeout: 30 * time.Second},
		userAgent: defaultUA,
		attempts:  defaultAttempts,
		backoff:   defaultBackoff,
		sleep:     time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// wikiResponse is the MediaWiki envelope. Failures arrive as an error object
// with HTTP 200, so success is asserted on the parsed text, never on status.
type wikiResponse struct {
	Parse struct {
		Text struct {
			Content string `json:"*"`
		} `json:"text"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// FetchPatches returns the patch calendar, newest first.
//
// It retries once on the failures that a refetch can plausibly fix — transport
// errors, HTTP 429 and 5xx, and a body that arrived without tables — and never
// on the ones it cannot: a 4xx, a MediaWiki error envelope, or a page whose
// structure genuinely changed.
func (c *Client) FetchPatches(ctx context.Context) ([]domain.Patch, error) {
	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		patches, retryAfter, err := c.fetchOnce(ctx)
		if err == nil {
			return patches, nil
		}
		lastErr = err
		if !retryable(err) || attempt == c.attempts {
			break
		}
		wait := c.backoff
		if retryAfter > wait {
			wait = retryAfter
		}
		slog.Warn("retrying liquipedia fetch", "attempt", attempt, "wait", wait, "err", err)
		c.sleep(wait)
	}
	return nil, lastErr
}

// retryable reports whether refetching could plausibly change the outcome.
func retryable(err error) bool {
	var pe *ParseError
	if errors.As(err, &pe) {
		return pe.Retryable()
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.Status == http.StatusTooManyRequests || se.Status >= 500
	}
	// Transport errors: the request never produced a response.
	return !errors.As(err, new(*wikiError))
}

// statusError reports a non-200 response.
type statusError struct{ Status int }

func (e *statusError) Error() string { return fmt.Sprintf("liquipedia status %d", e.Status) }

// wikiError reports a MediaWiki error envelope, which arrives with HTTP 200.
type wikiError struct{ Code, Info string }

func (e *wikiError) Error() string {
	return fmt.Sprintf("liquipedia error %s: %s", e.Code, e.Info)
}

// fetchOnce performs a single request, also returning any Retry-After the
// server asked for.
func (c *Client) fetchOnce(ctx context.Context) ([]domain.Patch, time.Duration, error) {
	q := url.Values{
		"action": {"parse"},
		"page":   {patchesPage},
		"format": {"json"},
		"prop":   {"text"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	// Accept-Encoding is deliberately not set here. Liquipedia requires gzip,
	// and Go's transport both offers it and decompresses transparently — but
	// only when it set the header itself. Setting it by hand yields raw gzip
	// bytes on the response body.

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch patches: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, retryAfterOf(resp), &statusError{Status: resp.StatusCode}
	}

	var wire wikiResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, 0, fmt.Errorf("decode liquipedia response: %w", err)
	}
	if wire.Error != nil {
		return nil, 0, &wikiError{Code: wire.Error.Code, Info: wire.Error.Info}
	}
	if wire.Parse.Text.Content == "" {
		return nil, 0, &ParseError{}
	}
	patches, err := ParsePatches(strings.NewReader(wire.Parse.Text.Content))
	return patches, 0, err
}

// retryAfterOf reads the server's own backoff request, in seconds.
func retryAfterOf(resp *http.Response) time.Duration {
	secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
