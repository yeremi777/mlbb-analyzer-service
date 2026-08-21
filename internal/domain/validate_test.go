package domain

import (
	"strings"
	"testing"
)

func hero(uid string, mlid int) Hero {
	return Hero{UID: uid, MLID: mlid, Name: uid, Roles: []string{"tank"}}
}

func TestValidateHeroes(t *testing.T) {
	tests := []struct {
		name    string
		heroes  []Hero
		wantErr bool
	}{
		{"valid", []Hero{hero("miya", 1), hero("tigreal", 2)}, false},
		{"empty set", nil, true},
		{"duplicate uid", []Hero{hero("miya", 1), hero("miya", 2)}, true},
		{"empty uid", []Hero{hero("", 1)}, true},
		{"non-positive mlid", []Hero{hero("miya", 0)}, true},
		{"no roles", []Hero{{UID: "miya", MLID: 1, Name: "Miya"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateHeroes(tt.heroes); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHeroes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func matchup(first, second, proofID string) Matchup {
	return Matchup{First: first, Second: second, Proof: []Proof{{ID: proofID}}}
}

func TestValidateMatchups(t *testing.T) {
	tests := []struct {
		name    string
		ms      []Matchup
		wantErr bool
	}{
		{"valid", []Matchup{matchup("tigreal", "diggie", "p1"), matchup("miya", "saber", "p2")}, false},
		{"empty set", nil, true},
		{"duplicate pair", []Matchup{matchup("tigreal", "diggie", "p1"), matchup("tigreal", "diggie", "p2")}, true},
		{"empty hero id", []Matchup{matchup("tigreal", "", "p1")}, true},
		{"hero paired with itself", []Matchup{matchup("tigreal", "tigreal", "p1")}, true},
		{"no proof", []Matchup{{First: "tigreal", Second: "diggie"}}, true},
		{"empty proof id", []Matchup{matchup("tigreal", "diggie", "")}, true},
		// A proof id is unique across every matchup, not just within one pair:
		// the database keys proofs by id alone.
		{"proof id reused across pairs",
			[]Matchup{matchup("tigreal", "diggie", "same"), matchup("miya", "saber", "same")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateMatchups(tt.ms); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMatchups() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHeroReferences(t *testing.T) {
	roster := []Hero{hero("tigreal", 1), hero("diggie", 2)}
	tests := []struct {
		name    string
		ms      []Matchup
		wantErr bool
	}{
		{"both known", []Matchup{matchup("tigreal", "diggie", "p1")}, false},
		{"unknown second", []Matchup{matchup("tigreal", "hirrara", "p1")}, true},
		{"unknown first", []Matchup{matchup("hirrara", "diggie", "p1")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateHeroReferences(roster, tt.ms); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHeroReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHeroReferencesNamesTheOffender(t *testing.T) {
	err := ValidateHeroReferences([]Hero{hero("tigreal", 1)},
		[]Matchup{matchup("tigreal", "hirrara", "p1")})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), `"hirrara"`) {
		t.Errorf("error should name the unknown id, got: %v", err)
	}
}
