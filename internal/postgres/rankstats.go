package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

const rankStatColumns = `main_hero_id, hero_uid, hero_name, rank_tier, window_days,
	snapshot_date, snapshot_age_days, win_rate, appearance_share, ban_rate,
	win_rate_delta, appearance_share_delta, ban_rate_delta, prev_snapshot_date,
	win_rate_rank, appearance_rank, patch_as_of, days_since_patch, window_crosses_patch`

// scanRankStat maps the nullable mart columns onto the domain type: an unnamed
// hero is one upstream carries and the dataset does not, which reads as an
// empty name rather than an error.
func scanRankStat(row pgx.Row) (domain.HeroRankStat, error) {
	var (
		s        domain.HeroRankStat
		uid, nam *string
	)
	err := row.Scan(&s.MainHeroID, &uid, &nam, &s.RankTier, &s.WindowDays,
		&s.SnapshotDate, &s.SnapshotAgeDays, &s.WinRate, &s.AppearanceShare, &s.BanRate,
		&s.WinRateDelta, &s.AppearanceShareDelta, &s.BanRateDelta, &s.PrevSnapshotDate,
		&s.WinRateRank, &s.AppearanceRank,
		&s.PatchAsOf, &s.DaysSincePatch, &s.WindowCrossesPatch)
	if err != nil {
		return s, err
	}
	if uid != nil {
		s.HeroUID = *uid
	}
	if nam != nil {
		s.HeroName = *nam
	}
	return s, nil
}

// HeroRankBoard returns every hero's current standing at one tier and window,
// best win rate first. A limit of 0 returns the whole board.
func HeroRankBoard(ctx context.Context, q Querier, tier string, window, limit int) ([]domain.HeroRankStat, error) {
	query := `SELECT ` + rankStatColumns + `
		FROM marts.hero_current
		WHERE rank_tier = $1 AND window_days = $2
		ORDER BY win_rate_rank`
	args := []any{tier, window}
	if limit > 0 {
		query += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("hero rank board %s/%dd: %w", tier, window, err)
	}
	defer rows.Close()

	var board []domain.HeroRankStat
	for rows.Next() {
		s, err := scanRankStat(rows)
		if err != nil {
			return nil, fmt.Errorf("scan rank stat: %w", err)
		}
		board = append(board, s)
	}
	return board, rows.Err()
}

// GetHeroRankStat returns one hero's current standing; pgx.ErrNoRows when the
// hero has no snapshot for that tier and window.
//
// It keys on the upstream hero id rather than the dataset uid: the mart carries
// heroes the dataset has no uid for, and the id is the only key that reaches
// all of them.
func GetHeroRankStat(ctx context.Context, q Querier, heroID int, tier string, window int) (domain.HeroRankStat, error) {
	return scanRankStat(q.QueryRow(ctx, `SELECT `+rankStatColumns+`
		FROM marts.hero_current
		WHERE main_hero_id = $1 AND rank_tier = $2 AND window_days = $3`, heroID, tier, window))
}

// HeroCounterStats returns the heroes that currently beat one hero at a tier
// and window, strongest first. A hero nothing is observed to beat yields an
// empty slice, not an error.
//
// It keys on the upstream hero id for the same reason GetHeroRankStat does: the
// mart carries heroes the dataset has no uid for.
func HeroCounterStats(ctx context.Context, q Querier, targetHeroID int, tier string, window int) ([]domain.HeroCounterStat, error) {
	rows, err := q.Query(ctx, `SELECT target_heroid, target_uid, target_name,
			counter_heroid, counter_uid, counter_name,
			win_rate_delta, source, source_rank, rank_tier, window_days, snapshot_date
		FROM marts.hero_counter_current
		WHERE target_heroid = $1 AND rank_tier = $2 AND window_days = $3
		ORDER BY win_rate_delta DESC, counter_heroid`, targetHeroID, tier, window)
	if err != nil {
		return nil, fmt.Errorf("hero counters %d %s/%dd: %w", targetHeroID, tier, window, err)
	}
	defer rows.Close()

	counters := []domain.HeroCounterStat{}
	for rows.Next() {
		var (
			c                        domain.HeroCounterStat
			tUID, tName, cUID, cName *string
		)
		if err := rows.Scan(&c.TargetHeroID, &tUID, &tName,
			&c.CounterHeroID, &cUID, &cName,
			&c.WinRateDelta, &c.Source, &c.SourceRank,
			&c.RankTier, &c.WindowDays, &c.SnapshotDate); err != nil {
			return nil, fmt.Errorf("scan counter stat: %w", err)
		}
		c.TargetUID, c.TargetName = deref(tUID), deref(tName)
		c.CounterUID, c.CounterName = deref(cUID), deref(cName)
		counters = append(counters, c)
	}
	return counters, rows.Err()
}

// deref reads a nullable text column: an unnamed hero is one upstream carries
// and the dataset does not, which reads as empty rather than as an error.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
