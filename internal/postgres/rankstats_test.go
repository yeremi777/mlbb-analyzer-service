package postgres

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"
)

// These read from marts.hero_current / marts.hero_synergy, which are views
// over whatever the collector has already stored. They assert shape and
// internal consistency rather than specific rates, which change daily.

func latestTierWindow(t *testing.T, tx pgx.Tx) (string, int) {
	t.Helper()
	var tier string
	var window int
	err := tx.QueryRow(context.Background(),
		`SELECT rank_tier, window_days FROM marts.hero_current
		  GROUP BY rank_tier, window_days ORDER BY count(*) DESC LIMIT 1`).Scan(&tier, &window)
	if err != nil {
		t.Skipf("marts.hero_current has no rows (run make migrate-up and make collect): %v", err)
	}
	return tier, window
}

func TestHeroRankBoardIsRankedAndComplete(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 0)
	if err != nil {
		t.Fatalf("HeroRankBoard: %v", err)
	}
	if len(board) == 0 {
		t.Fatal("want rows for a tier that exists in the mart")
	}

	// Ordered best win rate first, and the rank column must agree with it.
	for i, s := range board {
		if i > 0 && board[i-1].WinRate < s.WinRate {
			t.Fatalf("board not ordered by win rate at %d: %v then %v", i, board[i-1].WinRate, s.WinRate)
		}
		if s.RankTier != tier || s.WindowDays != window {
			t.Errorf("row %d has wrong combo: %s/%d", i, s.RankTier, s.WindowDays)
		}
		if s.MainHeroID == 0 {
			t.Errorf("row %d missing hero id", i)
		}
	}
	if board[0].WinRateRank != 1 {
		t.Errorf("first row should be win_rate_rank 1, got %d", board[0].WinRateRank)
	}
}

func TestHeroRankBoardLimit(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 5)
	if err != nil {
		t.Fatalf("HeroRankBoard: %v", err)
	}
	if len(board) != 5 {
		t.Fatalf("want 5 rows, got %d", len(board))
	}
}

func TestGetHeroRankStatRoundTrips(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := board[0]

	got, err := GetHeroRankStat(ctx, tx, want.MainHeroID, tier, window)
	if err != nil {
		t.Fatalf("GetHeroRankStat: %v", err)
	}
	if got.MainHeroID != want.MainHeroID || got.WinRate != want.WinRate {
		t.Errorf("round trip mismatch: got %+v want %+v", got, want)
	}
	if got.SnapshotDate.IsZero() {
		t.Error("snapshot date not scanned")
	}
	// A delta is either absent or paired with the date it is measured against.
	if (got.WinRateDelta == nil) != (got.PrevSnapshotDate == nil) {
		t.Errorf("delta and prev_snapshot_date disagree: delta=%v prev=%v",
			got.WinRateDelta, got.PrevSnapshotDate)
	}
}

func TestGetHeroRankStatUnknownHero(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	_, err := GetHeroRankStat(ctx, tx, 999999, tier, window)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("want pgx.ErrNoRows for an unknown hero, got %v", err)
	}
}

func TestHeroCounterStatsAreOrientedAndOrdered(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 1)
	if err != nil {
		t.Fatal(err)
	}
	hero := board[0].MainHeroID

	counters, err := HeroCounterStats(ctx, tx, hero, tier, window)
	if err != nil {
		t.Fatalf("HeroCounterStats: %v", err)
	}
	if len(counters) == 0 {
		t.Fatal("want counters for a hero present in the mart")
	}
	prev := math.Inf(1)
	for i, c := range counters {
		if c.TargetHeroID != hero {
			t.Errorf("row %d targets hero %d, want %d", i, c.TargetHeroID, hero)
		}
		if c.CounterHeroID == hero {
			t.Errorf("row %d has hero %d countering itself", i, hero)
		}
		// The orientation invariant: staging flips sub_hero_last rows so every
		// row reads "counter beats target", never the reverse.
		if c.WinRateDelta <= 0 {
			t.Errorf("row %d has non-positive delta %v: orientation is wrong", i, c.WinRateDelta)
		}
		if c.WinRateDelta > prev {
			t.Errorf("row %d delta %v is above the previous %v: not strongest first", i, c.WinRateDelta, prev)
		}
		prev = c.WinRateDelta
	}
}

func TestHeroCounterStatsUnknownHeroIsEmptyNotError(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	counters, err := HeroCounterStats(ctx, tx, 999999, tier, window)
	if err != nil {
		t.Fatalf("an unknown hero is an empty result, not an error: %v", err)
	}
	if len(counters) != 0 {
		t.Errorf("want no counters, got %d", len(counters))
	}
}

func TestCounterViewFlipsSubHeroLast(t *testing.T) {
	// Upstream reports both lists from the main hero's point of view. A
	// sub_hero row means "this hero beats main"; a sub_hero_last row means
	// "main beats this hero" and must come back with the ids swapped and the
	// sign flipped. Real history carries no sub_hero_last yet, so the payload
	// is synthetic.
	tx := testTx(t)
	ctx := context.Background()

	const (
		main    = 900001
		beatsUs = 900002
		weBeat  = 900003
	)
	payload := `{"data":{"main_heroid":900001,
		"sub_hero":[{"heroid":900002,"increase_win_rate":0.04}],
		"sub_hero_last":[{"heroid":900003,"increase_win_rate":-0.06}]}}`
	if _, err := tx.Exec(ctx, `
		INSERT INTO raw.hero_rank_snapshots
		  (main_heroid, rank_tier, window_days, snapshot_date, win_rate, appearance_share, ban_rate, payload)
		VALUES ($1, 'mythic', 7, DATE '2999-01-01', 0.5, 0.01, 0.02, $2::jsonb)`,
		main, payload); err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `
		SELECT target_heroid, counter_heroid, win_rate_delta, source
		  FROM staging.hero_counter_daily
		 WHERE snapshot_date = DATE '2999-01-01'
		 ORDER BY source`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type row struct {
		target, counter int
		delta           float64
		source          string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.target, &r.counter, &r.delta, &r.source); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []row{
		{target: main, counter: beatsUs, delta: 0.04, source: "sub_hero"},
		{target: weBeat, counter: main, delta: 0.06, source: "sub_hero_last"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].target != want[i].target || got[i].counter != want[i].counter ||
			got[i].source != want[i].source || math.Abs(got[i].delta-want[i].delta) > 1e-9 {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestHeroRankStatCarriesPatchContext(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 0)
	if err != nil {
		t.Fatal(err)
	}

	for i, s := range board {
		// An unknown patch is reported as unknown across all three fields,
		// never partially guessed.
		known := s.PatchAsOf != nil
		if (s.DaysSincePatch != nil) != known || (s.WindowCrossesPatch != nil) != known {
			t.Fatalf("row %d has partial patch context: as_of=%v days=%v crosses=%v",
				i, s.PatchAsOf, s.DaysSincePatch, s.WindowCrossesPatch)
		}
		if !known {
			continue
		}
		if *s.DaysSincePatch < 0 {
			t.Errorf("row %d: days_since_patch negative (%d) — patch is in the future",
				i, *s.DaysSincePatch)
		}
		// The flag must agree with the arithmetic it summarises.
		wantCrosses := *s.DaysSincePatch < s.WindowDays
		if *s.WindowCrossesPatch != wantCrosses {
			t.Errorf("row %d: window_crosses_patch=%v but days_since=%d window=%d",
				i, *s.WindowCrossesPatch, *s.DaysSincePatch, s.WindowDays)
		}
	}
}
