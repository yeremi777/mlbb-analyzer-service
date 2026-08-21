package dataset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twoHeroes = `[
		{"uid":"tigreal","mlid":"6","name":"Tigreal","roles":["tank"],"lanes":["roam"]},
		{"uid":"diggie","mlid":"48","name":"Diggie","roles":["support"],"lanes":["roam"]}
	]`

// writeMini lays out a complete dataset directory: heroes plus one counter file
// and one synergy file, so Load has every input it needs.
func writeMini(t *testing.T, heroes, counterRow, synergyRow string) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"counters", "synergies"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"heroes.json":            heroes,
		"counters.json":          `{"files":["counters/tigreal.json"]}`,
		"counters/tigreal.json":  counterRow,
		"synergies.json":         `{"files":["synergies/tigreal.json"]}`,
		"synergies/tigreal.json": synergyRow,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func counterRow(second string) string {
	return `[{"targetHeroId":"tigreal","counterHeroId":"` + second + `","reasons":["r"],
		"counterTypes":["anti-cc"],"proof":[{"id":"c1","category":"skill-interaction",
		"priority":"primary","impact":"high","summary":"s"}]}]`
}

func synergyRow(second string) string {
	return `[{"anchorHeroId":"tigreal","synergyHeroId":"` + second + `","reasons":["r"],
		"synergyTypes":["engage"],"proof":[{"id":"s1","category":"skill-interaction",
		"priority":"primary","impact":"high","summary":"s"}]}]`
}

func TestLoadReal(t *testing.T) {
	ds, err := Load(filepath.Join("..", "..", "data", "static"))
	if err != nil {
		t.Fatalf("load real dataset: %v", err)
	}
	if len(ds.Heroes) != 132 || len(ds.Counters) != 660 || len(ds.Synergies) != 660 {
		t.Fatalf("got %d heroes, %d counters, %d synergies",
			len(ds.Heroes), len(ds.Counters), len(ds.Synergies))
	}
}

func TestLoadAcceptsKnownHeroes(t *testing.T) {
	dir := writeMini(t, twoHeroes, counterRow("diggie"), synergyRow("diggie"))
	if _, err := Load(dir); err != nil {
		t.Fatalf("want success, got %v", err)
	}
}

func TestLoadRejectsUnknownCounterHero(t *testing.T) {
	// The typo the foreign key would otherwise report as a constraint name.
	dir := writeMini(t, twoHeroes, counterRow("hirrara"), synergyRow("diggie"))
	err := loadMustFail(t, dir)
	if !strings.Contains(err.Error(), "counters.json") || !strings.Contains(err.Error(), `"hirrara"`) {
		t.Errorf("error should name the file and the unknown id, got: %v", err)
	}
}

func TestLoadRejectsUnknownSynergyHero(t *testing.T) {
	dir := writeMini(t, twoHeroes, counterRow("diggie"), synergyRow("hirrara"))
	err := loadMustFail(t, dir)
	if !strings.Contains(err.Error(), "synergies.json") || !strings.Contains(err.Error(), `"hirrara"`) {
		t.Errorf("error should name the file and the unknown id, got: %v", err)
	}
}

// loadMustFail loads dir and returns the error, failing the test if there is none.
func loadMustFail(t *testing.T, dir string) error {
	t.Helper()
	if _, err := Load(dir); err != nil {
		return err
	}
	t.Fatal("want error, got nil")
	return nil
}
