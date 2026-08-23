package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

// The guard keeps an unchanged patch from being rewritten, so fetched_at marks
// when a patch last changed rather than when it was last seen.
const insertPatch = `INSERT INTO raw.patch_snapshots (release_date, version, highlights)
	VALUES ($1, $2, COALESCE($3::text[], '{}'))
	ON CONFLICT (release_date, version) DO UPDATE
	   SET highlights = EXCLUDED.highlights,
	       fetched_at = now()
	 WHERE raw.patch_snapshots.highlights IS DISTINCT FROM EXCLUDED.highlights`

// InsertPatches stores one fetch of the patch calendar and reports how many
// rows it changed. An unchanged calendar reports zero.
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

	changed := 0
	for i := range patches {
		tag, err := results.Exec()
		if err != nil {
			return changed, fmt.Errorf("insert patch %s: %w", patches[i].Version, err)
		}
		changed += int(tag.RowsAffected())
	}
	if err := results.Close(); err != nil {
		return changed, fmt.Errorf("close patch batch: %w", err)
	}
	return changed, nil
}
