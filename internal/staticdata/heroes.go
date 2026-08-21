package staticdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Hero struct {
	UID   string   `json:"uid"`
	MLID  int      `json:"mlid,string"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	Lanes []string `json:"lanes"`
	// Open-ended image URLs by variant, currently "head".
	Images json.RawMessage `json:"images" swaggertype:"object"`
}

// LoadHeroes reads heroes.json from dataDir and validates the invariants the
// database cannot see before a sync: a non-empty list (an empty source must
// never wipe the dimension) and unique, non-empty identity fields.
func LoadHeroes(dataDir string) ([]Hero, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, "heroes.json"))
	if err != nil {
		return nil, fmt.Errorf("read heroes.json: %w", err)
	}

	var heroes []Hero
	if err := json.Unmarshal(raw, &heroes); err != nil {
		return nil, fmt.Errorf("parse heroes.json: %w", err)
	}
	if len(heroes) == 0 {
		return nil, fmt.Errorf("heroes.json contains no heroes; refusing to sync an empty source")
	}

	seen := make(map[string]struct{}, len(heroes))
	for i, h := range heroes {
		if h.UID == "" || h.MLID <= 0 || h.Name == "" {
			return nil, fmt.Errorf("hero at index %d has empty uid, name, or non-positive mlid", i)
		}
		if len(h.Roles) == 0 {
			return nil, fmt.Errorf("hero %q has no roles", h.UID)
		}
		if _, dup := seen[h.UID]; dup {
			return nil, fmt.Errorf("duplicate hero uid %q", h.UID)
		}
		seen[h.UID] = struct{}{}
		if len(h.Images) == 0 {
			heroes[i].Images = json.RawMessage("{}")
		}
	}
	return heroes, nil
}
