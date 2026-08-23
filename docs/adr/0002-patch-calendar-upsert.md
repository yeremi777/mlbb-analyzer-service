# 0002 — The patch calendar is a dimension, not a snapshot stream

## Status

Accepted. Supersedes the append-only decision recorded in
`migrations/0010_raw_patch_snapshots.sql` for `raw.patch_snapshots` only.
`raw.hero_rank_snapshots` is unaffected.

## Context

`raw.patch_snapshots` was built to the same rule as `raw.hero_rank_snapshots`:
every fetch appends, nothing is ever updated or deleted, and a staging view
resolves repeated fetches to the newest answer. The rule earns its place for
hero statistics. Win rate, appearance share and ban rate are genuinely
different every day, so each fetch is a new fact and the table is a time series.

The patch calendar is not that. Liquipedia's Portal:Patches lists 113 releases
going back to 2019 and gains one entry every few weeks. The collector fetches
the whole page daily at 07:00 and appended all 113 rows each time, so the table
grew by 113 rows a day to record that nothing had changed — roughly 41,000 rows
a year, of which about 30 carry new information.

Measured on the live table, a second run reproduces the first exactly:

    rows 113 -> 226, distinct (release_date, version) pairs 113 -> 113
    pairs whose highlights differ between the two fetches: 0

`staging.patch_calendar` collapsed the duplicates with `DISTINCT ON
(release_date)`, so nothing downstream ever saw them and the growth went
unnoticed until the raw table was read directly.

The append-only rule was applied here by analogy with the hero snapshots rather
than because the calendar needed it.

## Decision

`raw.patch_snapshots` carries a unique index on `(release_date, version)` and
the collector upserts into it:

    ON CONFLICT (release_date, version) DO UPDATE
       SET highlights = EXCLUDED.highlights, fetched_at = now()
     WHERE raw.patch_snapshots.highlights IS DISTINCT FROM EXCLUDED.highlights

The `WHERE` guard is load-bearing. Without it every run rewrites all 113 rows
and `fetched_at` churns daily to record that nothing changed. With it, an
unchanged patch costs no write, so `fetched_at` means "when this patch last
changed" instead of "when we last looked", and `InsertPatches` returns a row
count that is a real change signal.

### Why the key is `(release_date, version)`

Not `version` alone. Patches before 2025 reused a version number across
successive updates: 1.9.42 shipped five times between 2024-12-18 and
2025-02-13, 1.9.20 five times, and 38 of the 75 distinct versions on the page
repeat. Keying on version would reject 38 legitimate releases, truncate the
calendar from 113 entries to 75, and mis-attribute `patch_as_of` in
`marts.hero_current` for every snapshot in the affected windows.

Not `release_date` alone either, even though it is the natural key and is
unique across all 113 rows today. Including `version` means a corrected version
for a known date arrives as a new row rather than overwriting the answer it
replaces. `staging.patch_calendar` already resolves that pair by
`fetched_at DESC, id DESC`, so the correction wins and the superseded row stays
inspectable. The cost is one ignored row per correction, an event that has not
occurred in the source's recorded history.

### Why the staging view stays

`staging.patch_calendar` is unchanged. The unique key still permits two rows
for one release date when a version is corrected, so the view's `DISTINCT ON`
still does real work. Leaving it alone also avoids a cascading drop and
recreate of `marts.hero_current`, which depends on it.

## Consequences

The duplication stops. The table holds one row per released patch and grows
only when Moonton ships one.

A highlights edit on Liquipedia is now taken. Editors routinely fill in the
Release Highlights column over the days after a patch ships, so a patch
collected on release morning would otherwise keep whatever it had that day.
Rejecting the correction — an `ON CONFLICT DO NOTHING` — would freeze the
thinnest version of the truth permanently.

`highlights` is overwritten in place, which is what is given up. If Liquipedia
is vandalised or an editor makes a page worse, the previous text is gone rather
than staying on record. This is a real loss of the property
`migrations/0010_raw_patch_snapshots.sql` was written to protect, accepted
because a 113-row dimension refreshed daily from a public wiki is repaired by
the next run, and because partial protection — keeping version history while
overwriting highlights — is not a principle worth the table growth.

The `0013` migration deletes the accumulated duplicates before building the
index, keeping the newest fetch of each patch. Its Down migration drops the
index but cannot restore the deleted rows.
