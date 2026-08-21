package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

const heroesFile = "heroes.json"

// loadHeroes reads heroes.json from dataDir and returns the heroes it declares,
// once the domain has accepted them.
func loadHeroes(dataDir string) ([]domain.Hero, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, heroesFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", heroesFile, err)
	}

	var heroes []domain.Hero
	if err := json.Unmarshal(raw, &heroes); err != nil {
		return nil, fmt.Errorf("parse %s: %w", heroesFile, err)
	}
	for i := range heroes {
		// An omitted "images" key decodes to nil, which is not valid jsonb.
		if len(heroes[i].Images) == 0 {
			heroes[i].Images = json.RawMessage("{}")
		}
	}

	if err := domain.ValidateHeroes(heroes); err != nil {
		return nil, fmt.Errorf("%s: %w", heroesFile, err)
	}
	return heroes, nil
}
