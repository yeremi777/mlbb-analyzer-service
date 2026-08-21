package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
)

// SyncHeroes makes public.heroes match the given source exactly: rows absent
// from the source are deleted first (so a departed hero cannot collide with a
// survivor on a unique column like mlid), then present heroes are upserted,
// touching updated_at only on real change. Callers own the transaction, so a
// failure anywhere leaves the table untouched.
func SyncHeroes(ctx context.Context, tx pgx.Tx, heroes []staticdata.Hero) error {
	batch := &pgx.Batch{}
	uids := make([]string, len(heroes))
	for i, h := range heroes {
		uids[i] = h.UID
		batch.Queue(`
			INSERT INTO public.heroes (uid, mlid, name, roles, lanes, images)
			VALUES ($1, $2, $3, $4, $5, COALESCE($6::jsonb, '{}'))
			ON CONFLICT (uid) DO UPDATE
			SET mlid = EXCLUDED.mlid,
			    name = EXCLUDED.name,
			    roles = EXCLUDED.roles,
			    lanes = EXCLUDED.lanes,
			    images = EXCLUDED.images,
			    updated_at = now()
			WHERE (heroes.mlid, heroes.name, heroes.roles, heroes.lanes, heroes.images)
			      IS DISTINCT FROM
			      (EXCLUDED.mlid, EXCLUDED.name, EXCLUDED.roles, EXCLUDED.lanes, EXCLUDED.images)`,
			h.UID, h.MLID, h.Name, h.Roles, h.Lanes, []byte(h.Images),
		)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM public.heroes WHERE uid <> ALL($1)", uids); err != nil {
		return fmt.Errorf("delete heroes absent from source: %w", err)
	}

	results := tx.SendBatch(ctx, batch)
	for _, h := range heroes {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert hero %q: %w", h.UID, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close upsert batch: %w", err)
	}
	return nil
}
