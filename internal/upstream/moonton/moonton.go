// Package moonton implements the Moonton first-party hero-stats client used by
// the collector. It talks to the undocumented
// POST /api/gms/source/2669606/{endpointId} endpoint that
// www.mobilelegends.com uses, and is deliberately independent of the rest of
// the service so the collector stays a leaf.
package moonton

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WindowDays maps a trailing aggregate window (in days) to its upstream
// CMS endpoint id. These are content ids and are the fragile part of the
// upstream contract.
var WindowDays = map[int]int{
	1:  2756567,
	3:  2756568,
	7:  2756569,
	15: 2756565,
	30: 2756570,
}

// RankTier maps a tier name to the upstream bigrank filter value.
var RankTier = map[string]string{
	"all":    "101",
	"epic":   "5",
	"legend": "6",
	"mythic": "7",
	"honor":  "8",
	"glory":  "9",
}

const (
	projectID = "2669606"
	defaultUA = "mlbb-analyzer-collector/1.0 (+github.com/yeremi777/mlbb-analyzer-service; kuroganehunter99@gmail.com)"
)

// Combo is one (window, rank tier) pull.
type Combo struct {
	WindowDays int
	RankTier   string
	EndpointID int
	BigRank    string
}

// AllCombos returns the full 5 x 6 = 30 set of (window, tier) combinations,
// matching the spec's loop.
func AllCombos() []Combo {
	var out []Combo
	// Stable order: windows ascending, tiers in the canonical order.
	tiers := []string{"all", "epic", "legend", "mythic", "honor", "glory"}
	for _, d := range []int{1, 3, 7, 15, 30} {
		for _, tier := range tiers {
			out = append(out, Combo{WindowDays: d, RankTier: tier,
				EndpointID: WindowDays[d], BigRank: RankTier[tier]})
		}
	}
	return out
}

// Record is one hero's row inside the upstream response. Upstream wraps each
// hero's fields under a `data` object; the typed fields are decoded from there.
// The full raw record (including sub_hero) is preserved in Raw for the payload
// column.
type Record struct {
	MainHeroID      int             `json:"main_heroid"`
	WinRate         float64         `json:"main_hero_win_rate"`
	AppearanceShare float64         `json:"main_hero_appearance_rate"`
	BanRate         float64         `json:"main_hero_ban_rate"`
	Raw             json.RawMessage `json:"-"`
}

// wireRecord mirrors the per-record `data` wrapper.
type wireRecord struct {
	Data Record `json:"data"`
}

// Response is the upstream envelope. Records is populated during decoding;
// each Record.Raw holds the hero's full JSON object verbatim (including
// sub_hero), so callers can store the payload column without loss.
type Response struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Total   int      `json:"total"`
	Records []Record `json:"records"`
}

// wireResponse is the on-the-wire shape used to capture each record's raw
// bytes before typed decoding. Records and Total live under `data` (upstream
// nests them); Code/Message sit at the top level.
type wireResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total   int               `json:"total"`
		Records []json.RawMessage `json:"records"`
	} `json:"data"`
}

// Client fetches rank snapshots from Moonton.
type Client struct {
	baseURL   string
	http      *http.Client
	userAgent string
	origin    string
	referer   string
}

// New creates a Client. baseURL defaults to the live Moonton stats API.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:   "https://api.gms.moontontech.com/api/gms/source/" + projectID,
		http:      &http.Client{Timeout: 30 * time.Second},
		userAgent: defaultUA,
		origin:    "https://www.mobilelegends.com",
		referer:   "https://www.mobilelegends.com/",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the upstream base URL (for tests/stubs).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient overrides the HTTP client (for tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithUserAgent overrides the polite User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// EndpointURL returns the full URL for a combo.
func (c *Client) EndpointURL(combo Combo) string {
	return fmt.Sprintf("%s/%d", c.baseURL, combo.EndpointID)
}

// Fetch one (window, tier) combination. It returns the decoded response plus
// the search-with-sign behaviour: a response with code != 0, or with
// records == nil / total == 0, is treated as a failure (ErrEmpty) rather than
// an empty-but-successful day — see docs/research/upstream-data-sources.md
// "Silent empty results".
func (c *Client) Fetch(ctx context.Context, combo Combo) (*Response, error) {
	if combo.EndpointID == 0 {
		return nil, fmt.Errorf("no endpoint id for window %d", combo.WindowDays)
	}
	if combo.BigRank == "" {
		return nil, fmt.Errorf("no bigrank for tier %q", combo.RankTier)
	}

	body := buildBody(combo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.EndpointURL(combo), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.origin)
	req.Header.Set("Referer", c.referer)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain a little for diagnostics.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream status %d for %s", resp.StatusCode, combo.Key())
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wire wireResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("decode upstream response for %s: %w", combo.Key(), err)
	}
	if wire.Code != 0 {
		return nil, &EmptyError{Combo: combo.Key(), Reason: fmt.Sprintf("code=%d message=%q", wire.Code, wire.Message)}
	}
	if len(wire.Data.Records) == 0 {
		return nil, &EmptyError{Combo: combo.Key(), Reason: "records=null/empty (silent empty upstream result)"}
	}
	// A short read is a partial page, not a smaller roster: storing it would
	// look like a clean run while dropping heroes. Stays a plain error so the
	// caller retries it, unlike EmptyError.
	if wire.Data.Total != len(wire.Data.Records) {
		return nil, fmt.Errorf("truncated response for %s: upstream total %d, got %d records",
			combo.Key(), wire.Data.Total, len(wire.Data.Records))
	}
	out := &Response{Code: wire.Code, Message: wire.Message, Total: wire.Data.Total, Records: make([]Record, len(wire.Data.Records))}
	for i, rec := range wire.Data.Records {
		var wr wireRecord
		if err := json.Unmarshal(rec, &wr); err != nil {
			return nil, fmt.Errorf("decode record %d for %s: %w", i, combo.Key(), err)
		}
		wr.Data.Raw = rec
		out.Records[i] = wr.Data
	}
	return out, nil
}

func buildBody(combo Combo) []byte {
	payload := map[string]any{
		"pageSize":  200,
		"pageIndex": 1,
		"filters": []map[string]string{
			{"field": "bigrank", "operator": "eq", "value": combo.BigRank},
			{"field": "match_type", "operator": "eq", "value": "0"},
		},
		"sorts": []map[string]any{
			{"data": map[string]string{"field": "main_hero_win_rate", "order": "desc"}, "type": "sequence"},
		},
		"fields": []string{
			"main_hero", "main_hero_appearance_rate", "main_hero_ban_rate",
			"main_hero_channel", "main_hero_win_rate", "main_heroid",
			"data.sub_hero.hero", "data.sub_hero.hero_channel",
			"data.sub_hero.increase_win_rate", "data.sub_hero.heroid",
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// Key returns a short human label for a combo: "7d/mythic".
func (c Combo) Key() string {
	return fmt.Sprintf("%dd/%s", c.WindowDays, c.RankTier)
}

// EmptyError reports an upstream response that looks like success but carries
// no data — the thing the research calls out as the dangerous silent case.
type EmptyError struct {
	Combo  string
	Reason string
}

func (e *EmptyError) Error() string {
	return fmt.Sprintf("empty upstream result for %s: %s", e.Combo, e.Reason)
}

// IsEmpty reports whether err is an EmptyError (useful with errors.As too).
func IsEmpty(err error) bool {
	var e *EmptyError
	return errors.As(err, &e)
}
