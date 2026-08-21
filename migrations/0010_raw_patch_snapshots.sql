-- +goose Up

-- The MLBB patch calendar as Liquipedia reported it, appended once per fetch.
--
-- Append-only for the same reason as hero snapshots: raw records what a source
-- said, not what we concluded. A vandalised or corrected wiki edit leaves the
-- previous answer intact and inspectable rather than overwriting it. Named
-- _snapshots to match raw.hero_rank_snapshots: both are point-in-time reads.
--
-- Not keyed on version: patches before 2025 reused a version number across
-- successive updates (1.9.42 shipped five times), so version is an attribute
-- of a release date, never its identity.
CREATE TABLE raw.patch_snapshots (
    id           BIGSERIAL   PRIMARY KEY,
    version      TEXT        NOT NULL,
    release_date DATE        NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX patch_snapshots_lookup_idx ON raw.patch_snapshots (release_date, fetched_at DESC);

-- One row per release date: the newest fetch wins, exactly as
-- staging.hero_rank_snapshot_latest resolves repeated hero snapshots.
CREATE VIEW staging.patch_calendar AS
SELECT DISTINCT ON (release_date)
       release_date, version, fetched_at
  FROM raw.patch_snapshots
 ORDER BY release_date, fetched_at DESC, id DESC;

-- +goose Down
DROP VIEW staging.patch_calendar;
DROP TABLE raw.patch_snapshots;
