package staticdata

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDataset(t *testing.T, index string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "counters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "counters.json"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const validMatchup = `[{"targetHeroId":"tigreal","counterHeroId":"diggie",
	"reasons":["r"],"counterTypes":["anti-cc"],
	"proof":[{"id":"p1","category":"skill-interaction","priority":"primary","impact":"high","summary":"s"}]}]`

func TestLoadCountersReal(t *testing.T) {
	ms, err := LoadCounters(filepath.Join("..", "..", "data", "static"))
	if err != nil {
		t.Fatalf("load real counters: %v", err)
	}
	if len(ms) != 660 {
		t.Fatalf("got %d counter matchups, want 660", len(ms))
	}
	proofs := 0
	for _, m := range ms {
		proofs += len(m.Proof)
	}
	if proofs != 660 {
		t.Fatalf("got %d proofs, want 660", proofs)
	}
}

func TestLoadSynergiesReal(t *testing.T) {
	ms, err := LoadSynergies(filepath.Join("..", "..", "data", "static"))
	if err != nil {
		t.Fatalf("load real synergies: %v", err)
	}
	if len(ms) != 660 {
		t.Fatalf("got %d synergy matchups, want 660", len(ms))
	}
}

func TestLoadCountersRejectsFilenameMismatch(t *testing.T) {
	dir := writeDataset(t,
		`{"files":["counters/miya.json"]}`,
		map[string]string{"counters/miya.json": validMatchup}) // targetHeroId is tigreal
	if _, err := LoadCounters(dir); err == nil {
		t.Fatal("want filename/target mismatch error, got nil")
	}
}

func TestLoadCountersRejectsDuplicatePair(t *testing.T) {
	dup := `[{"targetHeroId":"tigreal","counterHeroId":"diggie","reasons":["r"],"counterTypes":["t"],
		"proof":[{"id":"p1","category":"skill-interaction","priority":"primary","impact":"high","summary":"s"}]},
		{"targetHeroId":"tigreal","counterHeroId":"diggie","reasons":["r"],"counterTypes":["t"],
		"proof":[{"id":"p2","category":"skill-interaction","priority":"primary","impact":"high","summary":"s"}]}]`
	dir := writeDataset(t, `{"files":["counters/tigreal.json"]}`, map[string]string{"counters/tigreal.json": dup})
	if _, err := LoadCounters(dir); err == nil {
		t.Fatal("want duplicate pair error, got nil")
	}
}

func TestLoadCountersRejectsDuplicateProofID(t *testing.T) {
	a := `[{"targetHeroId":"tigreal","counterHeroId":"diggie","reasons":["r"],"counterTypes":["t"],
		"proof":[{"id":"same","category":"skill-interaction","priority":"primary","impact":"high","summary":"s"}]}]`
	b := `[{"targetHeroId":"miya","counterHeroId":"saber","reasons":["r"],"counterTypes":["t"],
		"proof":[{"id":"same","category":"skill-interaction","priority":"primary","impact":"high","summary":"s"}]}]`
	dir := writeDataset(t, `{"files":["counters/tigreal.json","counters/miya.json"]}`,
		map[string]string{"counters/tigreal.json": a, "counters/miya.json": b})
	if _, err := LoadCounters(dir); err == nil {
		t.Fatal("want duplicate proof id error, got nil")
	}
}
