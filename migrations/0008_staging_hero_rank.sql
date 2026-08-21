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

-- One row per (hero, partner) pairing, exploded out of the payload's sub_hero
-- array. Upstream returns five partners already ordered best-first, so
-- partner_rank carries upstream's own ranking.
--
-- win_rate_lift is upstream's increase_win_rate: an additive delta on the
-- main hero's win rate when the pair appears together, not a multiplier.
--
-- The CASE guards the array: payload is untrusted upstream JSON, and a
-- sub_hero that ever arrives as null or a scalar would otherwise error the
-- whole view rather than yield no pairs for that hero.
CREATE VIEW staging.hero_synergy_daily AS
SELECT l.main_hero_id,
       (s.elem->>'heroid')::integer                 AS partner_hero_id,
       (s.elem->>'increase_win_rate')::numeric(9,6) AS win_rate_lift,
       s.ord::smallint                              AS partner_rank,
       l.rank_tier,
       l.window_days,
       l.snapshot_date
  FROM staging.hero_rank_snapshot_latest l
  CROSS JOIN LATERAL jsonb_array_elements(
         CASE WHEN jsonb_typeof(l.payload->'data'->'sub_hero') = 'array'
              THEN l.payload->'data'->'sub_hero'
              ELSE '[]'::jsonb
         END) WITH ORDINALITY AS s(elem, ord)
 WHERE s.elem->>'heroid' IS NOT NULL;

-- +goose Down
DROP VIEW staging.hero_synergy_daily;
DROP VIEW staging.hero_rank_daily;
DROP VIEW staging.hero_rank_snapshot_latest;
