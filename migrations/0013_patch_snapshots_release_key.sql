-- +goose Up

-- One row per released patch, refreshed in place rather than appended per fetch.
--
-- The calendar is a slowly-changing dimension: 113 rows gaining one entry every
-- few weeks, re-read in full every morning. staging.patch_calendar hid the
-- resulting growth from readers.
--
-- The key is (release_date, version). Version alone is not unique — 38 of 75
-- versions repeat across release dates, 1.9.42 five times. Release date alone
-- would overwrite a corrected version instead of recording it.
--
-- See docs/adr/0002-patch-calendar-upsert.md.

-- Drops each row that has an older identical twin, keeping the fetch that first
-- carried the content so fetched_at stays truthful. A patch whose highlights
-- differ across fetches is left alone and fails the index build below: which
-- answer to keep is a judgement this migration must not make silently.
DELETE FROM raw.patch_snapshots a
      USING raw.patch_snapshots b
      WHERE a.release_date = b.release_date
        AND a.version = b.version
        AND a.highlights = b.highlights
        AND (b.fetched_at, b.id) < (a.fetched_at, a.id);

CREATE UNIQUE INDEX patch_snapshots_release_key
    ON raw.patch_snapshots (release_date, version);

-- +goose Down
DROP INDEX raw.patch_snapshots_release_key;
