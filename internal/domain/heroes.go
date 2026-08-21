// Package domain holds the game's vocabulary — heroes, matchups, proofs, rank
// statistics — and the invariants that hold no matter where the data came from. It performs no I/O and imports no other
// package in this service: everything else may depend on domain, and domain
// depends on none of them.
package domain

import (
	"encoding/json"
	"fmt"
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

// ValidateHeroes checks what no single row can: that the set is non-empty, so
// an empty source never wipes the dimension, and that identity is complete and
// unique across the set.
func ValidateHeroes(heroes []Hero) error {
	if len(heroes) == 0 {
		return fmt.Errorf("no heroes: refusing to sync an empty source")
	}

	seen := make(map[string]struct{}, len(heroes))
	for i, h := range heroes {
		if h.UID == "" || h.MLID <= 0 || h.Name == "" {
			return fmt.Errorf("hero at index %d has empty uid, name, or non-positive mlid", i)
		}
		if len(h.Roles) == 0 {
			return fmt.Errorf("hero %q has no roles", h.UID)
		}
		if _, dup := seen[h.UID]; dup {
			return fmt.Errorf("duplicate hero uid %q", h.UID)
		}
		seen[h.UID] = struct{}{}
	}
	return nil
}
