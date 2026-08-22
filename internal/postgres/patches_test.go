package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Far-future dates: the real calendar lives in the same table, so fixtures
// must not collide with a genuine release date.
func samplePatches() []domain.Patch {
	return []domain.Patch{
		{Version: "9.9.99", ReleaseDate: day(2999, time.August, 4)},
		{Version: "9.9.90", ReleaseDate: day(2999, time.July, 2)},
	}
}

func rawPatchCount(t *testing.T, tx pgx.Tx) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(),
		"SELECT count(*) FROM raw.patch_snapshots WHERE release_date > '2100-01-01'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func calendarCount(t *testing.T, tx pgx.Tx) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(),
		"SELECT count(*) FROM staging.patch_calendar WHERE release_date > '2100-01-01'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInsertPatchesAppends(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	n, err := InsertPatches(ctx, tx, samplePatches())
	if err != nil {
		t.Fatalf("InsertPatches: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 inserted, got %d", n)
	}
	if got := rawPatchCount(t, tx); got != 2 {
		t.Errorf("want 2 raw rows, got %d", got)
	}
}

func TestInsertPatchesIsAppendOnlyButCalendarDedupes(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := InsertPatches(ctx, tx, samplePatches()); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	// raw keeps every fetch...
	if got := rawPatchCount(t, tx); got != 6 {
		t.Errorf("want 6 raw rows after three fetches, got %d", got)
	}
	// ...while the calendar still expresses two facts.
	if got := calendarCount(t, tx); got != 2 {
		t.Errorf("want 2 calendar rows, got %d", got)
	}
}

func TestPatchCalendarTakesNewestFetch(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	if _, err := InsertPatches(ctx, tx, samplePatches()); err != nil {
		t.Fatal(err)
	}
	// Liquipedia corrects a mislabelled version for the same release date.
	corrected := []domain.Patch{{Version: "9.9.99a", ReleaseDate: day(2999, time.August, 4)}}
	if _, err := InsertPatches(ctx, tx, corrected); err != nil {
		t.Fatal(err)
	}

	var version string
	err := tx.QueryRow(ctx, "SELECT version FROM staging.patch_calendar WHERE release_date = $1",
		day(2999, time.August, 4)).Scan(&version)
	if err != nil {
		t.Fatal(err)
	}
	if version != "9.9.99a" {
		t.Errorf("calendar should show the newest fetch, got %q", version)
	}
	// The superseded answer is still on record.
	var raw int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM raw.patch_snapshots WHERE release_date = $1",
		day(2999, time.August, 4)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != 2 {
		t.Errorf("raw must keep both answers, got %d", raw)
	}
}

func TestInsertPatchesEmptyIsNoop(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	n, err := InsertPatches(ctx, tx, nil)
	if err != nil {
		t.Fatalf("InsertPatches: %v", err)
	}
	if n != 0 || rawPatchCount(t, tx) != 0 {
		t.Errorf("empty input must change nothing")
	}
}

func TestInsertPatchesStoresHighlights(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	want := []string{"Revamped Hero Kaja", "Hero Adjustments"}
	patches := []domain.Patch{
		{Version: "9.9.99", ReleaseDate: time.Date(2999, 1, 2, 0, 0, 0, 0, time.UTC), Highlights: want},
		// A patch the source lists no changes for: empty array, never NULL.
		{Version: "9.9.98", ReleaseDate: time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	if _, err := InsertPatches(ctx, tx, patches); err != nil {
		t.Fatal(err)
	}

	var got []string
	if err := tx.QueryRow(ctx,
		"SELECT highlights FROM raw.patch_snapshots WHERE version = '9.9.99'").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d highlights %q, want %q", len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("highlight %d = %q, want %q", i, got[i], want[i])
		}
	}

	var empty []string
	if err := tx.QueryRow(ctx,
		"SELECT highlights FROM raw.patch_snapshots WHERE version = '9.9.98'").Scan(&empty); err != nil {
		t.Fatalf("a patch with no highlights must store an empty array, not NULL: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("want empty highlights, got %q", empty)
	}
}
