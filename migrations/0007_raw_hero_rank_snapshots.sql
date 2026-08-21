-- +goose Up
CREATE SCHEMA IF NOT EXISTS raw;

CREATE TABLE raw.hero_rank_snapshots (
    id               BIGSERIAL PRIMARY KEY,
    main_heroid      INTEGER     NOT NULL,
    rank_tier        TEXT        NOT NULL,  -- all|epic|legend|mythic|honor|glory
    window_days      SMALLINT    NOT NULL,  -- 1|3|7|15|30
    snapshot_date    DATE        NOT NULL,  -- local date of the fetch
    win_rate         NUMERIC(9,6) NOT NULL,
    appearance_share NUMERIC(9,6) NOT NULL,
    ban_rate         NUMERIC(9,6) NOT NULL,
    payload          JSONB       NOT NULL,  -- full per-hero upstream record, unmodified (includes sub_hero)
    fetched_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX hero_rank_snapshots_lookup_idx
    ON raw.hero_rank_snapshots (main_heroid, rank_tier, window_days, snapshot_date);

-- +goose Down
DROP TABLE raw.hero_rank_snapshots;
