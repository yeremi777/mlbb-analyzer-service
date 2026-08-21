// Package collector orchestrates one daily collection run: it walks every
// (window, rank tier) combination, fetches each from Moonton, and appends the
// rows to the raw zone. A combo that fails is recorded and skipped; combos
// already stored keep their rows, so a partial run is never rolled back.
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/moonton"
)

// Fetcher retrieves one combo from upstream.
type Fetcher interface {
	Fetch(ctx context.Context, combo moonton.Combo) (*moonton.Response, error)
}

// Store persists a combo's records. Implementations own their transaction
// boundary: one committed transaction per combo, so a later failure cannot
// discard earlier work.
type Store interface {
	HasSnapshotDate(ctx context.Context, date string) (bool, error)
	InsertCombo(ctx context.Context, records []moonton.Record, combo moonton.Combo, date string) (int, error)
}

// Options configures a run. Combos, Attempts, Pause and Sleep have working
// defaults; Store and Fetcher are required.
type Options struct {
	Date     string
	Force    bool
	Combos   []moonton.Combo
	Pause    time.Duration
	Attempts int
	Backoff  time.Duration
	Store    Store
	Fetcher  Fetcher
	Sleep    func(time.Duration)
}

// Result summarises a run for logging and for the process exit code.
type Result struct {
	Date          string
	RowsInserted  int
	CombosFetched int
	Failed        []string
	Skipped       bool
}

const (
	defaultPause    = 2 * time.Second
	defaultAttempts = 2
	defaultBackoff  = 1 * time.Second
	backoffMax      = 10 * time.Second
)

func (o *Options) applyDefaults() {
	if len(o.Combos) == 0 {
		o.Combos = moonton.AllCombos()
	}
	if o.Pause == 0 {
		o.Pause = defaultPause
	}
	if o.Attempts == 0 {
		o.Attempts = defaultAttempts
	}
	if o.Backoff == 0 {
		o.Backoff = defaultBackoff
	}
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
}

// Run executes one collection pass. It returns a non-nil error when any combo
// failed, so cron sees a non-zero exit, while Result still reports what landed.
func Run(ctx context.Context, opts Options) (Result, error) {
	opts.applyDefaults()
	res := Result{Date: opts.Date}

	if !opts.Force {
		done, err := opts.Store.HasSnapshotDate(ctx, opts.Date)
		if err != nil {
			return res, fmt.Errorf("check existing snapshot for %s: %w", opts.Date, err)
		}
		if done {
			slog.Info("snapshot already collected, skipping", "date", opts.Date)
			res.Skipped = true
			return res, nil
		}
	}

	for i, combo := range opts.Combos {
		// Space every request, including the ones that just failed: a failing
		// upstream must not be hit 30 times back to back.
		if i > 0 {
			opts.Sleep(opts.Pause)
		}

		resp, err := fetchWithRetry(ctx, opts, combo)
		if err != nil {
			slog.Error("combo failed", "combo", combo.Key(), "err", err)
			res.Failed = append(res.Failed, combo.Key())
			continue
		}

		inserted, err := opts.Store.InsertCombo(ctx, resp.Records, combo, opts.Date)
		if err != nil {
			slog.Error("insert failed", "combo", combo.Key(), "err", err)
			res.Failed = append(res.Failed, combo.Key())
			continue
		}

		res.RowsInserted += inserted
		res.CombosFetched++
		slog.Info("combo stored", "combo", combo.Key(), "rows", inserted, "date", opts.Date)
	}

	if len(res.Failed) > 0 {
		return res, fmt.Errorf("collector finished with %d failed combos: %v", len(res.Failed), res.Failed)
	}
	return res, nil
}

// fetchWithRetry retries transport errors and truncated pages, but never an
// empty upstream result: that means the endpoint changed shape, and retrying
// only doubles the load while returning the same answer.
func fetchWithRetry(ctx context.Context, opts Options, combo moonton.Combo) (*moonton.Response, error) {
	backoff := opts.Backoff
	var lastErr error
	for attempt := 1; attempt <= opts.Attempts; attempt++ {
		resp, err := opts.Fetcher.Fetch(ctx, combo)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if moonton.IsEmpty(err) {
			return nil, err
		}
		if attempt < opts.Attempts {
			slog.Warn("retrying combo", "combo", combo.Key(), "attempt", attempt, "err", err)
			opts.Sleep(backoff)
			if backoff *= 2; backoff > backoffMax {
				backoff = backoffMax
			}
		}
	}
	return nil, lastErr
}
