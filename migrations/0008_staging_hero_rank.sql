-- +goose Up

-- Naming: raw mirrors upstream spelling (main_heroid is Moonton's field name),
-- staging and marts use house style (main_hero_id), matching public.counters
-- and public.synergies. Dates end _date, timestamps end _at.

-- One row per (hero, tier, window, date): the newest fetch wins. raw is
-- append-only, so a day re-collected three times has three copies of every
-- key; everything downstream reads through this seam, never raw directly.
CREATE VIEW staging.hero_rank_snapshot_latest AS
SELECT DISTINCT ON (main_heroid, rank_tier, window_days, snapshot_date)
       id,
       main_heroid AS main_hero_id,
       rank_tier, window_days, snapshot_date,
       win_rate, appearance_share, ban_rate, payload, fetched_at
  FROM raw.hero_rank_snapshots
 ORDER BY main_heroid, rank_tier, window_days, snapshot_date, fetched_at DESC, id DESC;

-- Deduped hero statistics, without the raw payload.
--
-- window_days is a trailing aggregate, so a row dated D with window_days = 1
-- describes roughly D-1, not D. snapshot_date records when the fetch ran.
CREATE VIEW staging.hero_rank_daily AS
SELECT main_hero_id,
       rank_tier,
       window_days,
       snapshot_date,
       win_rate,
       appearance_share,
       ban_rate,
       fetched_at
  FROM staging.hero_rank_snapshot_latest;

-- One row per directed counter matchup, exploded out of the payload's sub_hero
-- and sub_hero_last arrays and oriented so counter_heroid always beats
-- target_heroid by win_rate_delta.
--
-- Upstream reports this from the main hero's point of view: sub_hero holds the
-- five heroes with the largest positive win-rate delta against it, sub_hero_last
-- the five with the largest negative delta. The relation is antisymmetric --
-- across a full 133-hero pull, none of the 665 sub_hero pairs are mutual and 52%
-- appear in the partner's sub_hero_last -- so sub_hero_last rows are flipped and
-- negated rather than stored as a second kind of thing.
--
-- source keeps the two halves separable. Snapshots collected before the request
-- stopped narrowing its field list carry no sub_hero_last at all, and that
-- history cannot be backfilled.
--
-- The CASE guards each array: payload is untrusted upstream JSON, and a value
-- arriving as null or a scalar would otherwise error the whole view rather than
-- yield no rows for that hero.
CREATE VIEW staging.hero_counter_daily AS
SELECT l.main_hero_id                                  AS target_heroid,
       (s.elem->>'heroid')::integer                    AS counter_heroid,
       (s.elem->>'increase_win_rate')::numeric(9,6)    AS win_rate_delta,
       'sub_hero'::text                                AS source,
       s.ord::smallint                                 AS source_rank,
       l.rank_tier, l.window_days, l.snapshot_date
  FROM staging.hero_rank_snapshot_latest l
  CROSS JOIN LATERAL jsonb_array_elements(
         CASE WHEN jsonb_typeof(l.payload->'data'->'sub_hero') = 'array'
              THEN l.payload->'data'->'sub_hero'
              ELSE '[]'::jsonb
         END) WITH ORDINALITY AS s(elem, ord)
 WHERE s.elem->>'heroid' IS NOT NULL
   AND (s.elem->>'increase_win_rate')::numeric > 0

UNION ALL

-- The same relation seen from the losing end: the main hero counters these.
SELECT (s.elem->>'heroid')::integer                    AS target_heroid,
       l.main_hero_id                                  AS counter_heroid,
       -(s.elem->>'increase_win_rate')::numeric(9,6)   AS win_rate_delta,
       'sub_hero_last'::text                           AS source,
       s.ord::smallint                                 AS source_rank,
       l.rank_tier, l.window_days, l.snapshot_date
  FROM staging.hero_rank_snapshot_latest l
  CROSS JOIN LATERAL jsonb_array_elements(
         CASE WHEN jsonb_typeof(l.payload->'data'->'sub_hero_last') = 'array'
              THEN l.payload->'data'->'sub_hero_last'
              ELSE '[]'::jsonb
         END) WITH ORDINALITY AS s(elem, ord)
 WHERE s.elem->>'heroid' IS NOT NULL
   AND (s.elem->>'increase_win_rate')::numeric < 0;

-- +goose Down
DROP VIEW IF EXISTS staging.hero_counter_daily;
DROP VIEW IF EXISTS staging.hero_rank_daily;
DROP VIEW IF EXISTS staging.hero_rank_snapshot_latest;
