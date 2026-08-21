package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/moonton"
)

const testDate = "2999-01-01" // far future: never collides with real snapshots

func testCombo() moonton.Combo {
	return moonton.Combo{WindowDays: 7, RankTier: "mythic", EndpointID: 2756569, BigRank: "7"}
}

func snapshotRecords() []moonton.Record {
	return []moonton.Record{
		{MainHeroID: 60, WinRate: 0.528754, AppearanceShare: 0.034504, BanRate: 0.110542,
			Raw: json.RawMessage(`{"data":{"main_heroid":60,"sub_hero":[{"heroid":94,"increase_win_rate":0.02495}]}}`)},
		{MainHeroID: 132, WinRate: 0.589272, AppearanceShare: 0.001679, BanRate: 0.11355,
			Raw: json.RawMessage(`{"data":{"main_heroid":132,"sub_hero":[]}}`)},
	}
}

func snapshotCount(t *testing.T, tx pgx.Tx, date string) int {
	t.Helper()
	var n int
	err := tx.QueryRow(context.Background(),
		"SELECT count(*) FROM raw.hero_rank_snapshots WHERE snapshot_date = $1", date).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInsertSnapshotsWritesEveryRecord(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	n, err := InsertSnapshots(ctx, tx, snapshotRecords(), testCombo(), testDate)
	if err != nil {
		t.Fatalf("InsertSnapshots: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 inserted, got %d", n)
	}
	if got := snapshotCount(t, tx, testDate); got != 2 {
		t.Errorf("want 2 rows in table, got %d", got)
	}
}

func TestInsertSnapshotsPreservesPayloadVerbatim(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	if _, err := InsertSnapshots(ctx, tx, snapshotRecords(), testCombo(), testDate); err != nil {
		t.Fatalf("InsertSnapshots: %v", err)
	}

	var payload []byte
	err := tx.QueryRow(ctx,
		`SELECT payload FROM raw.hero_rank_snapshots WHERE snapshot_date = $1 AND main_heroid = 60`,
		testDate).Scan(&payload)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	inner, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("payload lost the data wrapper: %s", payload)
	}
	// sub_hero is the synergy evidence; losing it silently would be invisible.
	sub, ok := inner["sub_hero"].([]any)
	if !ok || len(sub) != 1 {
		t.Errorf("payload lost sub_hero: %s", payload)
	}
}

func TestInsertSnapshotsStoresTypedColumns(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	if _, err := InsertSnapshots(ctx, tx, snapshotRecords(), testCombo(), testDate); err != nil {
		t.Fatalf("InsertSnapshots: %v", err)
	}

	var (
		tier       string
		window     int
		win, share float64
	)
	err := tx.QueryRow(ctx,
		`SELECT rank_tier, window_days, win_rate, appearance_share
		   FROM raw.hero_rank_snapshots WHERE snapshot_date = $1 AND main_heroid = 60`,
		testDate).Scan(&tier, &window, &win, &share)
	if err != nil {
		t.Fatal(err)
	}
	if tier != "mythic" || window != 7 {
		t.Errorf("combo columns wrong: tier=%s window=%d", tier, window)
	}
	if win != 0.528754 || share != 0.034504 {
		t.Errorf("rates lost precision: win=%v share=%v", win, share)
	}
}

func TestInsertSnapshotsAppendsOnRerun(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := InsertSnapshots(ctx, tx, snapshotRecords(), testCombo(), testDate); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	// Append-only by design: no unique constraint rejects the second run.
	if got := snapshotCount(t, tx, testDate); got != 4 {
		t.Errorf("want 4 rows after two runs, got %d", got)
	}
}

func TestHasSnapshotDate(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	got, err := HasSnapshotDate(ctx, tx, testDate)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatalf("date %s should be empty before any insert", testDate)
	}

	if _, err := InsertSnapshots(ctx, tx, snapshotRecords(), testCombo(), testDate); err != nil {
		t.Fatal(err)
	}

	got, err = HasSnapshotDate(ctx, tx, testDate)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("want true after inserting rows for the date")
	}
}
