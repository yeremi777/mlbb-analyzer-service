package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

const insertPatch = `INSERT INTO raw.patch_snapshots (release_date, version, highlights)
	VALUES ($1, $2, COALESCE($3::text[], '{}'))`

// InsertPatches appends one fetch of the patch calendar and reports how many
// rows it wrote. Append-only by design: raw records what Liquipedia said, and
// staging.patch_calendar resolves repeated fetches to the newest answer, so a
// re-run adds rows without changing what downstream reads.
func InsertPatches(ctx context.Context, tx pgx.Tx, patches []domain.Patch) (int, error) {
	if len(patches) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, p := range patches {
		batch.Queue(insertPatch, p.ReleaseDate, p.Version, p.Highlights)
	}
	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	written := 0
	for i := range patches {
		if _, err := results.Exec(); err != nil {
			return written, fmt.Errorf("insert patch %s: %w", patches[i].Version, err)
		}
		written++
	}
	if err := results.Close(); err != nil {
		return written, fmt.Errorf("close patch batch: %w", err)
	}
	return written, nil
}
