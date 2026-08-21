package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
)

func testTx(t *testing.T) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "postgres://postgres:root@127.0.0.1:5432/mlbb_analyzer")
	if err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(ctx)
		_ = conn.Close(ctx)
	})
	return tx
}

func count(t *testing.T, tx pgx.Tx) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), "SELECT count(*) FROM public.heroes").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func heroes(uids ...string) []staticdata.Hero {
	out := make([]staticdata.Hero, len(uids))
	for i, uid := range uids {
		out[i] = staticdata.Hero{UID: uid, MLID: 1000 + i, Name: uid, Roles: []string{"tank"}, Lanes: []string{"roam"}}
	}
	return out
}

func TestSyncHeroesInsertsAndDeletes(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	if err := SyncHeroes(ctx, tx, heroes("a", "b", "c")); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if got := count(t, tx); got != 3 {
		t.Fatalf("after first sync: got %d rows, want 3", got)
	}

	// b removed from source: sync must delete it.
	if err := SyncHeroes(ctx, tx, heroes("a", "c")); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := count(t, tx); got != 2 {
		t.Fatalf("after removal sync: got %d rows, want 2", got)
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public.heroes WHERE uid='b')").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("hero b should have been deleted")
	}
}

func TestSyncHeroesIdempotent(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	hs := heroes("x", "y")
	if err := SyncHeroes(ctx, tx, hs); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := tx.QueryRow(ctx, "SELECT max(updated_at)::text FROM public.heroes").Scan(&before); err != nil {
		t.Fatal(err)
	}

	if err := SyncHeroes(ctx, tx, hs); err != nil {
		t.Fatal(err)
	}
	var after string
	if err := tx.QueryRow(ctx, "SELECT max(updated_at)::text FROM public.heroes").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("updated_at changed on identical re-sync: %s -> %s", before, after)
	}
	if got := count(t, tx); got != 2 {
		t.Fatalf("got %d rows, want 2", got)
	}
}

func TestSyncHeroesUpdatesChangedRow(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	hs := heroes("z")
	if err := SyncHeroes(ctx, tx, hs); err != nil {
		t.Fatal(err)
	}
	hs[0].Name = "Zeta"
	if err := SyncHeroes(ctx, tx, hs); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := tx.QueryRow(ctx, "SELECT name FROM public.heroes WHERE uid='z'").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Zeta" {
		t.Fatalf("got name %q, want Zeta", name)
	}
}
