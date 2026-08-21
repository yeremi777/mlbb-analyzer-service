package postgres

import (
	"context"
	"errors"
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

func TestHeroSynergyStatsOrderedByPartnerRank(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	board, err := HeroRankBoard(ctx, tx, tier, window, 1)
	if err != nil {
		t.Fatal(err)
	}
	hero := board[0].MainHeroID

	pairs, err := HeroSynergyStats(ctx, tx, hero, tier, window)
	if err != nil {
		t.Fatalf("HeroSynergyStats: %v", err)
	}
	if len(pairs) == 0 {
		t.Fatal("want synergy partners for a hero present in the mart")
	}
	for i, p := range pairs {
		if p.PartnerRank != i+1 {
			t.Errorf("partner %d has rank %d, want %d", i, p.PartnerRank, i+1)
		}
		if p.MainHeroID != hero {
			t.Errorf("partner %d belongs to hero %d, want %d", i, p.MainHeroID, hero)
		}
		if p.PartnerHeroID == p.MainHeroID {
			t.Errorf("hero %d paired with itself", hero)
		}
	}
}

func TestHeroSynergyStatsUnknownHeroIsEmptyNotError(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	tier, window := latestTierWindow(t, tx)

	pairs, err := HeroSynergyStats(ctx, tx, 999999, tier, window)
	if err != nil {
		t.Fatalf("an unknown hero is an empty result, not an error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("want no pairs, got %d", len(pairs))
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
