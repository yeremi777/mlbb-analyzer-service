package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "heroes.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// minHeroes guards against a truncated or half-written roster without pinning
// an exact count: heroes are added every few patches, and an exact number turns
// every roster change into a test failure that says nothing about correctness.
const minHeroes = 100

func TestLoadHeroesReal(t *testing.T) {
	heroes, err := loadHeroes(filepath.Join("..", "..", "data", "static"))
	if err != nil {
		t.Fatalf("load real dataset: %v", err)
	}
	if len(heroes) < minHeroes {
		t.Fatalf("got %d heroes, want at least %d: the roster looks truncated", len(heroes), minHeroes)
	}

	// Identity must hold for every hero, not just the first: a malformed entry
	// anywhere is what actually breaks the service.
	uids := make(map[string]bool, len(heroes))
	mlids := make(map[int]bool, len(heroes))
	for i, h := range heroes {
		if h.UID == "" || h.Name == "" || h.MLID <= 0 {
			t.Errorf("hero %d has incomplete identity: %+v", i, h)
		}
		if len(h.Roles) == 0 {
			t.Errorf("hero %q has no roles", h.UID)
		}
		if uids[h.UID] {
			t.Errorf("duplicate hero uid %q", h.UID)
		}
		if mlids[h.MLID] {
			t.Errorf("duplicate hero mlid %d (%s)", h.MLID, h.UID)
		}
		uids[h.UID], mlids[h.MLID] = true, true
	}
}

func TestLoadHeroesRejectsEmptyArray(t *testing.T) {
	dir := writeFile(t, `[]`)
	if _, err := loadHeroes(dir); err == nil {
		t.Fatal("want error for empty hero list, got nil")
	}
}

func TestLoadHeroesRejectsDuplicateUID(t *testing.T) {
	dir := writeFile(t, `[
		{"uid":"miya","mlid":"1","name":"Miya","roles":["marksman"]},
		{"uid":"miya","mlid":"2","name":"Miya2","roles":["marksman"]}
	]`)
	if _, err := loadHeroes(dir); err == nil {
		t.Fatal("want error for duplicate uid, got nil")
	}
}

func TestLoadHeroesRejectsMissingIdentity(t *testing.T) {
	dir := writeFile(t, `[{"uid":"","mlid":"1","name":"X","roles":["tank"]}]`)
	if _, err := loadHeroes(dir); err == nil {
		t.Fatal("want error for empty uid, got nil")
	}
}
