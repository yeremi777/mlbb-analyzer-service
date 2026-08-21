package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

func matchup(first, second, proofID string) domain.Matchup {
	return domain.Matchup{
		First: first, Second: second,
		Reasons: []string{"r"}, Types: []string{"t"},
		Proof: []domain.Proof{{
			ID: proofID, Category: "skill-interaction", Priority: "primary",
			Impact: "high", Summary: "s",
		}},
	}
}

func counts(t *testing.T, tx pgx.Tx, matchupTable, proofTable string) (int, int) {
	t.Helper()
	var m, p int
	if err := tx.QueryRow(context.Background(),
		"SELECT (SELECT count(*) FROM "+matchupTable+"), (SELECT count(*) FROM "+proofTable+")").Scan(&m, &p); err != nil {
		t.Fatal(err)
	}
	return m, p
}

func TestSyncCountersInsertsAndCascadesDelete(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	ms := []domain.Matchup{
		matchup("tigreal", "diggie", "t-p1"),
		matchup("tigreal", "valir", "t-p2"),
	}
	if err := SyncCounters(ctx, tx, ms); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if m, p := counts(t, tx, "public.counters", "public.counter_proofs"); m != 2 || p != 2 {
		t.Fatalf("got %d matchups %d proofs, want 2 2", m, p)
	}

	if err := SyncCounters(ctx, tx, ms[:1]); err != nil {
		t.Fatalf("removal sync: %v", err)
	}
	if m, p := counts(t, tx, "public.counters", "public.counter_proofs"); m != 1 || p != 1 {
		t.Fatalf("after removal: got %d matchups %d proofs, want 1 1 (proof must cascade)", m, p)
	}
}

func TestSyncCountersRejectsInvalidCategory(t *testing.T) {
	tx := testTx(t)
	bad := matchup("tigreal", "diggie", "t-p1")
	bad.Proof[0].Category = "not-a-category"
	if err := SyncCounters(context.Background(), tx, []domain.Matchup{bad}); err == nil {
		t.Fatal("want CHECK violation for invalid category, got nil")
	}
}

func TestSyncSynergiesIdempotent(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	ms := []domain.Matchup{matchup("tigreal", "pharsa", "s-p1")}
	if err := SyncSynergies(ctx, tx, ms); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := tx.QueryRow(ctx, "SELECT max(updated_at)::text FROM public.synergy_proofs").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := SyncSynergies(ctx, tx, ms); err != nil {
		t.Fatal(err)
	}
	var after string
	if err := tx.QueryRow(ctx, "SELECT max(updated_at)::text FROM public.synergy_proofs").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("proof updated_at churned on identical re-sync: %s -> %s", before, after)
	}
	if m, p := counts(t, tx, "public.synergies", "public.synergy_proofs"); m != 1 || p != 1 {
		t.Fatalf("got %d matchups %d proofs, want 1 1", m, p)
	}
}
