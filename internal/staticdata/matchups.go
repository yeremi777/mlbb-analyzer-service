package staticdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Proof struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Priority      string   `json:"priority"`
	Impact        string   `json:"impact"`
	Summary       string   `json:"summary"`
	WorksBestWhen []string `json:"worksBestWhen"`
	FailureCases  []string `json:"failureCases"`
}

// Matchup is one authored relation between two heroes. For counters First is
// the target and Second the counter; for synergies First is the anchor and
// Second the synergy partner. Types holds counterTypes or synergyTypes.
type Matchup struct {
	First   string
	Second  string
	Reasons []string
	Types   []string
	Proof   []Proof
}

type index struct {
	Files []string `json:"files"`
}

// LoadCounters reads counters.json plus every split file it lists, enforcing
// the file-level rules the database cannot see: the file name must match the
// matchup's target hero, pairs must be unique, proof ids must be globally
// unique, and the source must not be empty.
func LoadCounters(dataDir string) ([]Matchup, error) {
	return loadMatchups(dataDir, "counters.json", "targetHeroId", "counterHeroId", "counterTypes")
}

// LoadSynergies is LoadCounters for synergies.json, keyed on the anchor hero.
func LoadSynergies(dataDir string) ([]Matchup, error) {
	return loadMatchups(dataDir, "synergies.json", "anchorHeroId", "synergyHeroId", "synergyTypes")
}

func loadMatchups(dataDir, indexFile, firstKey, secondKey, typesKey string) ([]Matchup, error) {
	rawIndex, err := os.ReadFile(filepath.Join(dataDir, indexFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", indexFile, err)
	}
	var idx index
	if err := json.Unmarshal(rawIndex, &idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", indexFile, err)
	}
	if len(idx.Files) == 0 {
		return nil, fmt.Errorf("%s lists no files; refusing to sync an empty source", indexFile)
	}

	var matchups []Matchup
	pairs := make(map[[2]string]struct{})
	proofIDs := make(map[string]string)

	for _, rel := range idx.Files {
		raw, err := os.ReadFile(filepath.Join(dataDir, rel))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		stem := strings.TrimSuffix(filepath.Base(rel), ".json")

		for i, row := range rows {
			m := Matchup{}
			if err := pick(row, firstKey, &m.First); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}
			if err := pick(row, secondKey, &m.Second); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}
			if err := pick(row, "reasons", &m.Reasons); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}
			if err := pick(row, typesKey, &m.Types); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}
			if err := pick(row, "proof", &m.Proof); err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}

			if m.First != stem {
				return nil, fmt.Errorf("%s[%d]: %s %q does not match file name", rel, i, firstKey, m.First)
			}
			key := [2]string{m.First, m.Second}
			if _, dup := pairs[key]; dup {
				return nil, fmt.Errorf("%s: duplicate pair %s/%s", rel, m.First, m.Second)
			}
			pairs[key] = struct{}{}
			if len(m.Proof) == 0 {
				return nil, fmt.Errorf("%s[%d]: matchup has no proof", rel, i)
			}
			for _, p := range m.Proof {
				if p.ID == "" {
					return nil, fmt.Errorf("%s[%d]: proof with empty id", rel, i)
				}
				if prev, dup := proofIDs[p.ID]; dup {
					return nil, fmt.Errorf("%s: proof id %q already used in %s", rel, p.ID, prev)
				}
				proofIDs[p.ID] = rel
			}
			matchups = append(matchups, m)
		}
	}
	if len(matchups) == 0 {
		return nil, fmt.Errorf("%s: no matchups found; refusing to sync an empty source", indexFile)
	}
	return matchups, nil
}

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
