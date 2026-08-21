package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

// loadMatchups reads an index file plus every split file it lists. Each file is
// named for the hero it belongs to, and that name is authoritative: a row whose
// first hero id disagrees with its file name is an authoring mistake no
// downstream check could catch.
func loadMatchups(dataDir, indexFile, firstKey, secondKey, typesKey string) ([]domain.Matchup, error) {
	rawIndex, err := os.ReadFile(filepath.Join(dataDir, indexFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", indexFile, err)
	}
	var idx index
	if err := json.Unmarshal(rawIndex, &idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", indexFile, err)
	}
	if len(idx.Files) == 0 {
		return nil, fmt.Errorf("%s lists no files", indexFile)
	}

	var matchups []domain.Matchup
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
			m, err := decodeMatchup(row, firstKey, secondKey, typesKey)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", rel, i, err)
			}
			if m.First != stem {
				return nil, fmt.Errorf("%s[%d]: %s %q does not match file name", rel, i, firstKey, m.First)
			}
			matchups = append(matchups, m)
		}
	}

	if err := domain.ValidateMatchups(matchups); err != nil {
		return nil, fmt.Errorf("%s: %w", indexFile, err)
	}
	return matchups, nil
}

func decodeMatchup(row map[string]json.RawMessage, firstKey, secondKey, typesKey string) (domain.Matchup, error) {
	var m domain.Matchup
	fields := []struct {
		key string
		dst any
	}{
		{firstKey, &m.First},
		{secondKey, &m.Second},
		{"reasons", &m.Reasons},
		{typesKey, &m.Types},
		{"proof", &m.Proof},
	}
	for _, f := range fields {
		if err := pick(row, f.key, f.dst); err != nil {
			return m, err
		}
	}
	return m, nil
}

// loadCounters and loadSynergies name the two index files the dataset declares.
func loadCounters(dataDir string) ([]domain.Matchup, error) {
	return loadMatchups(dataDir, "counters.json", "targetHeroId", "counterHeroId", "counterTypes")
}

func loadSynergies(dataDir string) ([]domain.Matchup, error) {
	return loadMatchups(dataDir, "synergies.json", "anchorHeroId", "synergyHeroId", "synergyTypes")
}
