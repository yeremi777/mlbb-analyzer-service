-- +goose Up

-- What each patch changed, as Liquipedia's Release Highlights column lists it:
-- one array entry per bullet. Captured at collection time rather than fetched
-- on demand, because Liquipedia caps action=parse at one request per 30
-- seconds — a rate a report generator would exceed immediately.
--
-- Nullable would mean three states (absent, empty, listed) where the source has
-- two, so the column defaults to an empty array: 1 of the 113 patches currently
-- on the page genuinely lists no highlights.
--
-- staging.patch_calendar is deliberately left alone. marts.hero_current depends
-- on it, so widening it means a cascading drop and recreate of both views, and
-- nothing reads highlights yet. The migration that adds the first consumer can
-- pay that cost knowingly.
ALTER TABLE raw.patch_snapshots
    ADD COLUMN highlights TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE raw.patch_snapshots DROP COLUMN highlights;
