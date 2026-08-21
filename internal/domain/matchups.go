package domain

import "fmt"

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

// HeroMatchup is a Matchup with its partner resolved: Second carries the whole
// hero instead of an id. Matchup is the authored form read from the dataset;
// HeroMatchup is the form assembled for callers.
type HeroMatchup struct {
	First   string
	Second  Hero
	Reasons []string
	Types   []string
	Proof   []Proof
}

// ValidateMatchups checks what spans rows: a non-empty set, one relation per
// hero pair, and proof ids unique across every matchup, since the database keys
// a proof by its id alone regardless of which pair it belongs to.
func ValidateMatchups(ms []Matchup) error {
	if len(ms) == 0 {
		return fmt.Errorf("no matchups: refusing to sync an empty source")
	}

	pairs := make(map[[2]string]struct{}, len(ms))
	proofIDs := make(map[string][2]string)
	for _, m := range ms {
		if m.First == "" || m.Second == "" {
			return fmt.Errorf("matchup %q/%q has an empty hero id", m.First, m.Second)
		}
		if m.First == m.Second {
			return fmt.Errorf("matchup %s/%s pairs a hero with itself", m.First, m.Second)
		}
		pair := [2]string{m.First, m.Second}
		if _, dup := pairs[pair]; dup {
			return fmt.Errorf("duplicate pair %s/%s", m.First, m.Second)
		}
		pairs[pair] = struct{}{}

		if len(m.Proof) == 0 {
			return fmt.Errorf("matchup %s/%s has no proof", m.First, m.Second)
		}
		for _, p := range m.Proof {
			if p.ID == "" {
				return fmt.Errorf("matchup %s/%s has a proof with an empty id", m.First, m.Second)
			}
			if prev, dup := proofIDs[p.ID]; dup {
				return fmt.Errorf("proof id %q used by both %s/%s and %s/%s",
					p.ID, prev[0], prev[1], m.First, m.Second)
			}
			proofIDs[p.ID] = pair
		}
	}
	return nil
}

// ValidateHeroReferences checks that every hero id a matchup names exists in
// the hero set. The database enforces this with a foreign key; checking it
// against the authored data first names the offending pair and id instead of a
// constraint.
func ValidateHeroReferences(heroes []Hero, ms []Matchup) error {
	known := make(map[string]struct{}, len(heroes))
	for _, h := range heroes {
		known[h.UID] = struct{}{}
	}
	for _, m := range ms {
		for _, uid := range [2]string{m.First, m.Second} {
			if _, ok := known[uid]; !ok {
				return fmt.Errorf("matchup %s/%s references unknown hero %q",
					m.First, m.Second, uid)
			}
		}
	}
	return nil
}
