package collector

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/moonton"
)

// fakeStore records inserts in memory so the loop is testable without postgres.
type fakeStore struct {
	rows      map[string]int // combo key -> rows
	dates     map[string]bool
	insertErr map[string]error
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]int{}, dates: map[string]bool{}, insertErr: map[string]error{}}
}

func (f *fakeStore) HasSnapshotDate(_ context.Context, date string) (bool, error) {
	return f.dates[date], nil
}

func (f *fakeStore) InsertCombo(_ context.Context, records []moonton.Record, combo moonton.Combo, date string) (int, error) {
	if err := f.insertErr[combo.Key()]; err != nil {
		return 0, err
	}
	f.rows[combo.Key()] += len(records)
	f.dates[date] = true
	return len(records), nil
}

// fakeFetcher returns a canned response per combo, or an error.
type fakeFetcher struct {
	errs  map[string]error
	calls map[string]int
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{errs: map[string]error{}, calls: map[string]int{}}
}

func (f *fakeFetcher) Fetch(_ context.Context, combo moonton.Combo) (*moonton.Response, error) {
	f.calls[combo.Key()]++
	if err := f.errs[combo.Key()]; err != nil {
		return nil, err
	}
	return &moonton.Response{Code: 0, Total: 2, Records: []moonton.Record{
		{MainHeroID: 60, Raw: json.RawMessage(`{"data":{"main_heroid":60}}`)},
		{MainHeroID: 132, Raw: json.RawMessage(`{"data":{"main_heroid":132}}`)},
	}}, nil
}

func testCombos() []moonton.Combo {
	return []moonton.Combo{
		{WindowDays: 1, RankTier: "all", EndpointID: 2756567, BigRank: "101"},
		{WindowDays: 1, RankTier: "mythic", EndpointID: 2756567, BigRank: "7"},
		{WindowDays: 7, RankTier: "all", EndpointID: 2756569, BigRank: "101"},
	}
}

// sleepRecorder captures pause durations instead of actually sleeping.
type sleepRecorder struct{ durations []time.Duration }

func (s *sleepRecorder) sleep(d time.Duration) { s.durations = append(s.durations, d) }

func baseOpts(store Store, fetcher Fetcher, rec *sleepRecorder) Options {
	return Options{
		Date: "2026-08-22", Combos: testCombos(), Pause: 2 * time.Second,
		Attempts: 2, Backoff: time.Millisecond, Store: store, Fetcher: fetcher, Sleep: rec.sleep,
	}
}

func TestRunStoresEveryCombo(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	res, err := Run(context.Background(), baseOpts(store, fetcher, rec))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RowsInserted != 6 {
		t.Errorf("want 6 rows (3 combos x 2), got %d", res.RowsInserted)
	}
	if res.CombosFetched != 3 || len(res.Failed) != 0 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestRunPausesAfterFailedCombosToo(t *testing.T) {
	// Every combo fails. Without spacing on the failure path the collector
	// hammers a already-struggling upstream with back-to-back requests.
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	for _, c := range testCombos() {
		fetcher.errs[c.Key()] = &moonton.EmptyError{Combo: c.Key(), Reason: "records=null"}
	}

	if _, err := Run(context.Background(), baseOpts(store, fetcher, rec)); err == nil {
		t.Fatal("expected error when every combo fails")
	}

	var pauses int
	for _, d := range rec.durations {
		if d == 2*time.Second {
			pauses++
		}
	}
	// 3 combos -> at least one inter-combo pause per failure boundary.
	if pauses < 2 {
		t.Errorf("failures were not spaced: got %d pauses of 2s in %v", pauses, rec.durations)
	}
}

func TestRunKeepsSuccessfulCombosWhenOneFails(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	fetcher.errs["1d/mythic"] = errors.New("connection reset")

	res, err := Run(context.Background(), baseOpts(store, fetcher, rec))
	if err == nil {
		t.Fatal("expected non-zero outcome when a combo fails")
	}
	if res.RowsInserted != 4 {
		t.Errorf("successful combos must survive: want 4 rows, got %d", res.RowsInserted)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "1d/mythic" {
		t.Errorf("want 1d/mythic recorded as failed, got %v", res.Failed)
	}
	if store.rows["1d/mythic"] != 0 {
		t.Errorf("failed combo must not have stored rows")
	}
}

func TestRunRetriesTransientErrorButNotEmpty(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	fetcher.errs["1d/all"] = errors.New("connection reset")
	fetcher.errs["7d/all"] = &moonton.EmptyError{Combo: "7d/all", Reason: "records=null"}

	if _, err := Run(context.Background(), baseOpts(store, fetcher, rec)); err == nil {
		t.Fatal("expected error")
	}
	if got := fetcher.calls["1d/all"]; got != 2 {
		t.Errorf("transient error should be retried once: got %d attempts", got)
	}
	if got := fetcher.calls["7d/all"]; got != 1 {
		t.Errorf("empty upstream must not be retried: got %d attempts", got)
	}
}

func TestRunSkipsWhenDateAlreadyCollected(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	store.dates["2026-08-22"] = true

	res, err := Run(context.Background(), baseOpts(store, fetcher, rec))
	if err != nil {
		t.Fatalf("an already-collected date is a no-op, not an error: %v", err)
	}
	if !res.Skipped {
		t.Error("want Skipped=true")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("guard must run before any fetch, got calls %v", fetcher.calls)
	}
}

func TestRunForceOverridesGuard(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	store.dates["2026-08-22"] = true

	opts := baseOpts(store, fetcher, rec)
	opts.Force = true
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Skipped || res.RowsInserted != 6 {
		t.Errorf("force must collect anyway: %+v", res)
	}
}

func TestRunReportsInsertFailureSeparately(t *testing.T) {
	store, fetcher, rec := newFakeStore(), newFakeFetcher(), &sleepRecorder{}
	store.insertErr["7d/all"] = errors.New("deadlock detected")

	res, err := Run(context.Background(), baseOpts(store, fetcher, rec))
	if err == nil || !strings.Contains(err.Error(), "7d/all") {
		t.Fatalf("want error naming the failed combo, got %v", err)
	}
	if res.RowsInserted != 4 {
		t.Errorf("other combos must still be stored, got %d", res.RowsInserted)
	}
}
