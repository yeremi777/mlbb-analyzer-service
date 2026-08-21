package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

type matchupTables struct {
	matchup   string // qualified matchup table
	proof     string // qualified proof table
	firstCol  string
	secondCol string
	typesCol  string
}

var counterTables = matchupTables{
	matchup: "public.counters", proof: "public.counter_proofs",
	firstCol: "target_hero_id", secondCol: "counter_hero_id", typesCol: "counter_types",
}

var synergyTables = matchupTables{
	matchup: "public.synergies", proof: "public.synergy_proofs",
	firstCol: "anchor_hero_id", secondCol: "synergy_hero_id", typesCol: "synergy_types",
}

// SyncCounters makes counters and counter_proofs match the source
// exactly, with the same delete-first-then-upsert semantics as SyncHeroes.
// Deleting a matchup cascades to its proofs.
func SyncCounters(ctx context.Context, tx pgx.Tx, ms []domain.Matchup) error {
	return syncMatchups(ctx, tx, counterTables, ms)
}

// SyncSynergies is SyncCounters for synergies and synergy_proofs.
func SyncSynergies(ctx context.Context, tx pgx.Tx, ms []domain.Matchup) error {
	return syncMatchups(ctx, tx, synergyTables, ms)
}

func syncMatchups(ctx context.Context, tx pgx.Tx, t matchupTables, ms []domain.Matchup) error {
	firsts := make([]string, len(ms))
	seconds := make([]string, len(ms))
	var proofIDs []string
	for i, m := range ms {
		firsts[i], seconds[i] = m.First, m.Second
		for _, p := range m.Proof {
			proofIDs = append(proofIDs, p.ID)
		}
	}

	// Deletes first: a departed matchup cascades its proofs, and a proof moved
	// between matchups cannot collide with its old row.
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		"DELETE FROM %s WHERE id <> ALL($1)", t.proof), proofIDs); err != nil {
		return fmt.Errorf("delete departed proofs from %s: %w", t.proof, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		"DELETE FROM %s WHERE (%s, %s) NOT IN (SELECT unnest($1::text[]), unnest($2::text[]))",
		t.matchup, t.firstCol, t.secondCol), firsts, seconds); err != nil {
		return fmt.Errorf("delete departed matchups from %s: %w", t.matchup, err)
	}

	upsertMatchup := fmt.Sprintf(`
		INSERT INTO %s (%s, %s, reasons, %s)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (%s, %s) DO UPDATE
		SET reasons = EXCLUDED.reasons, %s = EXCLUDED.%s, updated_at = now()
		WHERE (%s.reasons, %s.%s) IS DISTINCT FROM (EXCLUDED.reasons, EXCLUDED.%s)`,
		t.matchup, t.firstCol, t.secondCol, t.typesCol,
		t.firstCol, t.secondCol,
		t.typesCol, t.typesCol,
		t.matchup, t.matchup, t.typesCol, t.typesCol)

	upsertProof := fmt.Sprintf(`
		INSERT INTO %s (id, %s, %s, category, priority, impact, summary, works_best_when, failure_cases)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8::text[], '{}'), COALESCE($9::text[], '{}'))
		ON CONFLICT (id) DO UPDATE
		SET %s = EXCLUDED.%s, %s = EXCLUDED.%s, category = EXCLUDED.category,
		    priority = EXCLUDED.priority, impact = EXCLUDED.impact, summary = EXCLUDED.summary,
		    works_best_when = EXCLUDED.works_best_when, failure_cases = EXCLUDED.failure_cases,
		    updated_at = now()
		WHERE (%s.%s, %s.%s, %s.category, %s.priority, %s.impact, %s.summary, %s.works_best_when, %s.failure_cases)
		      IS DISTINCT FROM
		      (EXCLUDED.%s, EXCLUDED.%s, EXCLUDED.category, EXCLUDED.priority, EXCLUDED.impact, EXCLUDED.summary, EXCLUDED.works_best_when, EXCLUDED.failure_cases)`,
		t.proof, t.firstCol, t.secondCol,
		t.firstCol, t.firstCol, t.secondCol, t.secondCol,
		t.proof, t.firstCol, t.proof, t.secondCol, t.proof, t.proof, t.proof, t.proof, t.proof, t.proof,
		t.firstCol, t.secondCol)

	batch := &pgx.Batch{}
	queued := 0
	for _, m := range ms {
		batch.Queue(upsertMatchup, m.First, m.Second, m.Reasons, m.Types)
		queued++
		for _, p := range m.Proof {
			batch.Queue(upsertProof, p.ID, m.First, m.Second, p.Category, p.Priority,
				p.Impact, p.Summary, p.WorksBestWhen, p.FailureCases)
			queued++
		}
	}
	results := tx.SendBatch(ctx, batch)
	for i := 0; i < queued; i++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert into %s/%s: %w", t.matchup, t.proof, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close upsert batch for %s: %w", t.matchup, err)
	}
	return nil
}
