// Package dataset reads the authored MLBB dataset from disk: heroes.json, and
// the counter and synergy index files together with the per-hero files they
// list. It owns the file layout — where files live, how they are named, how a
// row decodes — and hands the decoded values to the domain to check. No other package
// in the service touches these files.
package dataset

import (
	"encoding/json"
	"fmt"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

// Dataset is the authored dataset read as one unit. Counters and synergies name
// heroes, so they are only meaningful alongside the hero list they reference.
type Dataset struct {
	Heroes    []domain.Hero
	Counters  []domain.Matchup
	Synergies []domain.Matchup
}

// Load reads the whole dataset from dataDir and returns it only once every file
// is internally consistent and every hero a matchup names exists. It is the
// only way to read the dataset: loading the files apart would let a caller hold
// matchups whose references were never checked.
func Load(dataDir string) (*Dataset, error) {
	heroes, err := loadHeroes(dataDir)
	if err != nil {
		return nil, err
	}
	counters, err := loadCounters(dataDir)
	if err != nil {
		return nil, err
	}
	synergies, err := loadSynergies(dataDir)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateHeroReferences(heroes, counters); err != nil {
		return nil, fmt.Errorf("counters.json: %w", err)
	}
	if err := domain.ValidateHeroReferences(heroes, synergies); err != nil {
		return nil, fmt.Errorf("synergies.json: %w", err)
	}
	return &Dataset{Heroes: heroes, Counters: counters, Synergies: synergies}, nil
}

// index is the shape of counters.json and synergies.json: a list of per-hero
// files, relative to the dataset directory.
type index struct {
	Files []string `json:"files"`
}

// pick decodes one required field, so a missing key and a mistyped key are
// reported apart from each other.
func pick(row map[string]json.RawMessage, key string, dst any) error {
	raw, ok := row[key]
	if !ok {
		return fmt.Errorf("missing %q", key)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("field %q: %w", key, err)
	}
	return nil
}
