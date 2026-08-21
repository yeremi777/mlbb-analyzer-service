package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
)

// Querier is the read-only surface shared by pgxpool.Pool, pgx.Conn, and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const heroColumns = "uid, mlid, name, roles, lanes, images"

func scanHero(row pgx.Row) (staticdata.Hero, error) {
	var h staticdata.Hero
	err := row.Scan(&h.UID, &h.MLID, &h.Name, &h.Roles, &h.Lanes, &h.Images)
	return h, err
}

// ListHeroes returns every hero ordered by mlid, matching the authored file order.
func ListHeroes(ctx context.Context, q Querier) ([]staticdata.Hero, error) {
	rows, err := q.Query(ctx, "SELECT "+heroColumns+" FROM public.heroes ORDER BY mlid")
	if err != nil {
		return nil, fmt.Errorf("list heroes: %w", err)
	}
	defer rows.Close()

	var heroes []staticdata.Hero
	for rows.Next() {
		h, err := scanHero(rows)
		if err != nil {
			return nil, fmt.Errorf("scan hero: %w", err)
		}
		heroes = append(heroes, h)
	}
	return heroes, rows.Err()
}

// GetHero returns one hero by uid; pgx.ErrNoRows when absent.
func GetHero(ctx context.Context, q Querier, uid string) (staticdata.Hero, error) {
	return scanHero(q.QueryRow(ctx, "SELECT "+heroColumns+" FROM public.heroes WHERE uid = $1", uid))
}

// HeroMatchup is one counter or synergy relation with the partner hero joined in.
type HeroMatchup struct {
	First   string
	Second  staticdata.Hero
	Reasons []string
	Types   []string
	Proof   []staticdata.Proof
}

// CountersForTarget returns the target hero's counter matchups with proofs,
// ordered by counter hero id.
func CountersForTarget(ctx context.Context, q Querier, target string) ([]HeroMatchup, error) {
	return matchupsFor(ctx, q, counterTables, target)
}

// SynergiesForAnchor returns the anchor hero's synergy pairings with proofs,
// ordered by synergy hero id.
func SynergiesForAnchor(ctx context.Context, q Querier, anchor string) ([]HeroMatchup, error) {
	return matchupsFor(ctx, q, synergyTables, anchor)
}

func matchupsFor(ctx context.Context, q Querier, t matchupTables, first string) ([]HeroMatchup, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(
		`SELECT m.%s, m.reasons, m.%s, %s
		 FROM %s m JOIN public.heroes h ON h.uid = m.%s
		 WHERE m.%s = $1 ORDER BY m.%s`,
		t.secondCol, t.typesCol, heroColumnsAliased("h"),
		t.matchup, t.secondCol, t.firstCol, t.secondCol), first)
	if err != nil {
		return nil, fmt.Errorf("list matchups from %s: %w", t.matchup, err)
	}
	defer rows.Close()

	var ms []HeroMatchup
	byPartner := map[string]int{}
	for rows.Next() {
		m := HeroMatchup{First: first}
		var partner string
		if err := rows.Scan(&partner, &m.Reasons, &m.Types,
			&m.Second.UID, &m.Second.MLID, &m.Second.Name, &m.Second.Roles, &m.Second.Lanes, &m.Second.Images); err != nil {
			return nil, fmt.Errorf("scan matchup: %w", err)
		}
		m.Proof = []staticdata.Proof{}
		byPartner[partner] = len(ms)
		ms = append(ms, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return ms, nil
	}

	proofRows, err := q.Query(ctx, fmt.Sprintf(
		`SELECT %s, id, category, priority, impact, summary, works_best_when, failure_cases
		 FROM %s WHERE %s = $1 ORDER BY id`,
		t.secondCol, t.proof, t.firstCol), first)
	if err != nil {
		return nil, fmt.Errorf("list proofs from %s: %w", t.proof, err)
	}
	defer proofRows.Close()

	for proofRows.Next() {
		var partner string
		var p staticdata.Proof
		if err := proofRows.Scan(&partner, &p.ID, &p.Category, &p.Priority, &p.Impact,
			&p.Summary, &p.WorksBestWhen, &p.FailureCases); err != nil {
			return nil, fmt.Errorf("scan proof: %w", err)
		}
		if i, ok := byPartner[partner]; ok {
			ms[i].Proof = append(ms[i].Proof, p)
		}
	}
	return ms, proofRows.Err()
}

func heroColumnsAliased(alias string) string {
	return fmt.Sprintf("%s.uid, %s.mlid, %s.name, %s.roles, %s.lanes, %s.images",
		alias, alias, alias, alias, alias, alias)
}
