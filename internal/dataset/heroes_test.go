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

func TestLoadHeroesReal(t *testing.T) {
	heroes, err := loadHeroes(filepath.Join("..", "..", "data", "static"))
	if err != nil {
		t.Fatalf("load real dataset: %v", err)
	}
	if len(heroes) != 132 {
		t.Fatalf("got %d heroes, want 132", len(heroes))
	}
	if heroes[0].UID == "" || heroes[0].MLID <= 0 || heroes[0].Name == "" {
		t.Fatalf("first hero has empty identity: %+v", heroes[0])
	}
	if len(heroes[0].Roles) == 0 {
		t.Fatalf("first hero has no roles")
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
