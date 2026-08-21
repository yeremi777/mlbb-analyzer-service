-- +goose Up

-- Current standing of every hero, one row per (hero, tier, window), carrying
-- the movement since the previous snapshot so callers never compute deltas.
--
-- staging holds history; marts answers "right now". A caller wanting a series
-- reads staging.hero_rank_daily instead. Mart views state their grain, hence
-- the _current suffix.
--
-- Deltas compare against the previous *available* snapshot, not a fixed offset:
-- a missed collection day would otherwise silently produce a NULL delta rather
-- than a correct comparison against the last day that exists. prev_snapshot_date
-- is exposed so a caller can say what the movement is measured against, and is
-- NULL on a hero's first-ever snapshot.
--
-- The join to public.heroes is LEFT: upstream carries heroes the dataset does
-- not yet know (currently heroid 133), and dropping them would hide a live hero.
CREATE VIEW marts.hero_current AS
WITH movement AS (
    SELECT d.main_hero_id,
           d.rank_tier,
           d.window_days,
           d.snapshot_date,
           d.win_rate,
           d.appearance_share,
           d.ban_rate,
           lag(d.win_rate)         OVER w AS prev_win_rate,
           lag(d.appearance_share) OVER w AS prev_appearance_share,
           lag(d.ban_rate)         OVER w AS prev_ban_rate,
           lag(d.snapshot_date)    OVER w AS prev_snapshot_date,
           row_number() OVER (PARTITION BY d.main_hero_id, d.rank_tier, d.window_days
                              ORDER BY d.snapshot_date DESC) AS recency
      FROM staging.hero_rank_daily d
    WINDOW w AS (PARTITION BY d.main_hero_id, d.rank_tier, d.window_days
                 ORDER BY d.snapshot_date)
)
SELECT m.main_hero_id,
       h.uid                                    AS hero_uid,
       h.name                                   AS hero_name,
       m.rank_tier,
       m.window_days,
       m.snapshot_date,
       (CURRENT_DATE - m.snapshot_date)          AS snapshot_age_days,
       m.win_rate,
       m.appearance_share,
       m.ban_rate,
       m.win_rate         - m.prev_win_rate         AS win_rate_delta,
       m.appearance_share - m.prev_appearance_share AS appearance_share_delta,
       m.ban_rate         - m.prev_ban_rate         AS ban_rate_delta,
       m.prev_snapshot_date,
       rank() OVER (PARTITION BY m.rank_tier, m.window_days ORDER BY m.win_rate DESC)         AS win_rate_rank,
       rank() OVER (PARTITION BY m.rank_tier, m.window_days ORDER BY m.appearance_share DESC) AS appearance_rank
  FROM movement m
  LEFT JOIN public.heroes h ON h.mlid = m.main_hero_id
 WHERE m.recency = 1;

-- Current synergy pairings with both heroes named, best partner first.
--
-- win_rate_lift is an additive delta on the main hero's win rate when the pair
-- appears together, carried through from upstream unchanged.
CREATE VIEW marts.hero_synergy_current AS
WITH latest AS (
    SELECT s.*,
           dense_rank() OVER (PARTITION BY s.main_hero_id, s.rank_tier, s.window_days
                              ORDER BY s.snapshot_date DESC) AS recency
      FROM staging.hero_synergy_daily s
)
SELECT l.main_hero_id,
       mh.name AS hero_name,
       l.partner_hero_id,
       ph.uid  AS partner_uid,
       ph.name AS partner_name,
       l.win_rate_lift,
       l.partner_rank,
       l.rank_tier,
       l.window_days,
       l.snapshot_date
  FROM latest l
  LEFT JOIN public.heroes mh ON mh.mlid = l.main_hero_id
  LEFT JOIN public.heroes ph ON ph.mlid = l.partner_hero_id
 WHERE l.recency = 1;

-- +goose Down
DROP VIEW marts.hero_synergy_current;
DROP VIEW marts.hero_current;
