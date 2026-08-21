-- +goose Up

-- Adds patch context to the current-standing mart.
--
-- patch_as_of is the patch live on snapshot_date. days_since_patch measures
-- from snapshot_date, not from today, so the value does not drift.
--
-- window_crosses_patch is the honest-answer column: the statistics are
-- trailing-window aggregates, so a 7-day number collected 4 days after a patch
-- still contains 3 pre-patch days. It is true exactly when the window reaches
-- back before the current patch shipped, which tells a caller the number
-- cannot yet be attributed to that patch alone.
--
-- All three are NULL for a snapshot older than the earliest known patch: an
-- unknown patch is reported as unknown, never guessed.
DROP VIEW marts.hero_current;

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
       p.version                                AS patch_as_of,
       (m.snapshot_date - p.release_date)        AS days_since_patch,
       ((m.snapshot_date - m.window_days) < p.release_date) AS window_crosses_patch,
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
  LEFT JOIN LATERAL (
       SELECT pc.version, pc.release_date
         FROM staging.patch_calendar pc
        WHERE pc.release_date <= m.snapshot_date
        ORDER BY pc.release_date DESC
        LIMIT 1
  ) p ON TRUE
 WHERE m.recency = 1;

-- +goose Down
DROP VIEW marts.hero_current;

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
