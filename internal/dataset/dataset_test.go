package dataset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
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

// indexedSlugs returns the hero slug of every file an index lists, e.g.
// "counters/akai.json" -> "akai".
func indexedSlugs(t *testing.T, dataDir, indexFile string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, indexFile))
	if err != nil {
		t.Fatal(err)
	}
	var idx index
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatal(err)
	}
	slugs := make(map[string]bool, len(idx.Files))
	for _, f := range idx.Files {
		slugs[strings.TrimSuffix(filepath.Base(f), ".json")] = true
	}
	return slugs
}

// subjects returns the first hero of every matchup: the hero whose file it came
// from.
func subjects(ms []domain.Matchup) map[string]bool {
	out := make(map[string]bool, len(ms))
	for _, m := range ms {
		out[m.First] = true
	}
	return out
}

// assertIndexFullyLoaded checks the loader turned every indexed file into
// matchups, and that no file on disk is missing from the index. An unindexed
// file is the dangerous case: the loader never opens it, so its matchups are
// silently absent rather than reported as an error.
func assertIndexFullyLoaded(t *testing.T, dataDir, indexFile, dir string, loaded map[string]bool) {
	t.Helper()
	indexed := indexedSlugs(t, dataDir, indexFile)

	for slug := range indexed {
		if !loaded[slug] {
			t.Errorf("%s lists %q but no matchups were loaded for it", indexFile, slug)
		}
	}
	for slug := range loaded {
		if !indexed[slug] {
			t.Errorf("matchups loaded for %q which %s does not list", slug, indexFile)
		}
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		slug := strings.TrimSuffix(e.Name(), ".json")
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !indexed[slug] {
			t.Errorf("%s/%s exists on disk but %s does not list it, so it never loads",
				dir, e.Name(), indexFile)
		}
	}
}

// TestLoadReal asserts what must hold for any roster rather than today's
// counts: adding a hero or authoring new matchups must not fail this test.
func TestLoadReal(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data", "static")
	ds, err := Load(dataDir)
	if err != nil {
		t.Fatalf("load real dataset: %v", err)
	}
	if len(ds.Heroes) < minHeroes {
		t.Fatalf("got %d heroes, want at least %d", len(ds.Heroes), minHeroes)
	}

	assertIndexFullyLoaded(t, dataDir, "counters.json", "counters", subjects(ds.Counters))
	assertIndexFullyLoaded(t, dataDir, "synergies.json", "synergies", subjects(ds.Synergies))
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
