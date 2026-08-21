package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/upstream/moonton"
)

const insertSnapshot = `INSERT INTO raw.hero_rank_snapshots
    (main_heroid, rank_tier, window_days, snapshot_date, win_rate, appearance_share, ban_rate, payload)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// InsertSnapshots appends rows to raw.hero_rank_snapshots in one batch. It is
// append-only by design (see docs/specs/collector-raw-zone.md): re-runs add
// rows rather than replace, and no unique constraint rejects duplicates.
func InsertSnapshots(ctx context.Context, tx pgx.Tx, rows []moonton.Record, combo moonton.Combo, snapshotDate string) (int, error) {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(insertSnapshot, r.MainHeroID, combo.RankTier, combo.WindowDays,
			snapshotDate, r.WinRate, r.AppearanceShare, r.BanRate, r.Raw)
	}
	results := tx.SendBatch(ctx, batch)
	defer results.Close()
	inserted := 0
	for i := 0; i < len(rows); i++ {
		if _, err := results.Exec(); err != nil {
			return inserted, fmt.Errorf("insert snapshot %s record %d: %w", combo.Key(), i, err)
		}
		inserted++
	}
	if err := results.Close(); err != nil {
		return inserted, fmt.Errorf("close snapshot batch for %s: %w", combo.Key(), err)
	}
	return inserted, nil
}

const hasSnapshotDate = `SELECT EXISTS (SELECT 1 FROM raw.hero_rank_snapshots WHERE snapshot_date = $1)`

// HasSnapshotDate reports whether any rows already exist for a snapshot date.
// It is the guard that makes a second cron attempt on the same day a no-op.
func HasSnapshotDate(ctx context.Context, q Querier, date string) (bool, error) {
	var exists bool
	if err := q.QueryRow(ctx, hasSnapshotDate, date).Scan(&exists); err != nil {
		return false, fmt.Errorf("check snapshot date %s: %w", date, err)
	}
	return exists, nil
}

// SnapshotStore adapts these functions to the collector's Store role: one
// committed transaction per combo, so a combo that fails later in a run cannot
// discard the combos that already landed.
type SnapshotStore struct{ DB Executor }

func (s SnapshotStore) HasSnapshotDate(ctx context.Context, date string) (bool, error) {
	return HasSnapshotDate(ctx, s.DB, date)
}

func (s SnapshotStore) InsertCombo(ctx context.Context, records []moonton.Record, combo moonton.Combo, date string) (int, error) {
	var inserted int
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		n, err := InsertSnapshots(ctx, tx, records, combo, date)
		inserted = n
		return err
	})
	if err != nil {
		// The transaction rolled back: nothing landed, whatever the batch counted.
		return 0, err
	}
	return inserted, nil
}
