# empirical-evidence-in-scoring — draft

## Destination

The AI scorer reasons from measured play alongside authored proof: a synergy score
can be corroborated or contradicted by what the pairing actually does in ranked
games, and the model can finally speak about patch recency instead of being told
not to invent it. Reaching the end means a settled spec for which measured facts
enter which prompt, under what identity and freshness rules, and what the score
and confidence axes are then allowed to mean.

## Decisions so far

- [Role of measured data] — evidence for the AI, not a new API surface. The work
  lands in `internal/analyzer` and its prompt; `internal/api` gains no route.
  `marts.hero_current` and `marts.hero_synergy` are read through the existing
  handlers, not exposed directly.
- [`sub_hero` is counter data] — the opposite of what two earlier revisions of
  this spec recorded. mobilelegends.com renders its COUNTER HERO column from
  `sub_hero`, fetched with the field list this collector already used, and the
  relation is antisymmetric: of 665 directed pairs none are mutual and 52% appear
  in the partner's `sub_hero_last`. Counters therefore have pairwise measurement
  and synergies have none — the reverse of the assumption this spec was built on.
  See `sub-hero-is-counters`.
- [Collector field list] — the request sends `"fields": []` and takes the
  server's default projection. An explicit list already cost this project
  `sub_hero_last` and nine per-duration win rates for the collector's whole
  life, and omitted fields cannot be back-filled. Measured cost is roughly a
  third more stored bytes, not four times, because the larger payload crosses
  the TOAST threshold and is compressed.
- [Both matchup directions are available] — `sub_hero_last` is the same relation
  seen from the losing end: the five heroes the main hero counters. Normalised in
  `staging.hero_counter_daily`, an authored counter claim can now be corroborated
  or contradicted by measurement, and so can its absence.

## Open decisions

### Counter prompt evidence · Mode: HITL

Reopened on new ground. This decision assumed counters had only per-hero
standing, which fit neither `score` nor `confidence`; that assumption is gone.
Counters now have a measured, directed, pairwise delta — the strongest evidence
available anywhere in this project, and a direct match for an authored counter
claim. The live question is no longer whether counters can be grounded but how a
measured delta interacts with an authored proof: whether the two are averaged,
whether measurement can overturn an authored claim it contradicts, and what
happens when a hand-authored counter does not appear in the measured matrix at
all.

### Tier and window selection · Mode: HITL

Every measured fact is keyed on `(rank_tier, window_days)` — six tiers, five
windows. The analyze requests carry neither today. Either the service fixes one
combination, or the request contract grows the parameters and the frontend must
choose. A mythic 7-day number and an all-tier 1-day number describe different
games.

### Cache key under changing evidence · Mode: AFK

`HasCachedScore(kind, heroID, language)` keys a cached score on nothing that
changes when statistics change. Once measured facts enter the prompt, yesterday's
cached score can be served against today's numbers, and a cache hit after a patch
release would answer with pre-patch reasoning. Resolve what the key must include —
snapshot date, tier, window, patch version — and what that does to the hit rate
the cache exists to protect.

### Sample size floor for duration buckets · Mode: AFK

Every pairing entry carries win rate bucketed by game length
(`min_win_rate6` … `min_win_rate20`, nine buckets), which maps onto the authored
`game-phase` proof category. Observed values include `0`, `0.5` and `0.8` —
plainly one-, two- and five-game samples. Resolve the floor below which a bucket
is withheld from the prompt rather than presented as a fact, and whether the
endpoint exposes any count to compute it from.

## Not yet specified

- What the scorer does when a hero has measured statistics but no authored proof,
  or the reverse. The mart carries heroes the dataset does not know (heroid 133
  today) with a NULL `hero_uid`.
- Whether `window_crosses_patch` should suppress a measured fact outright or
  qualify it, and whether the model can be trusted to honour a qualification.
- Whether a measured matchup that contradicts an authored counter surfaces to
  the user, feeds the scorer silently, or becomes a dataset-authoring report.
- What synergy scoring does now that no measured synergy exists at all.
- Whether a stale snapshot (`snapshot_age_days` high after failed collection)
  degrades a score, blocks measured facts from the prompt, or is merely reported.

## Out of scope

- Collector control endpoints. Cron schedules, `flock` serialises, logs report;
  an HTTP trigger buys an auth, concurrency and long-request problem for nothing.
- A meta or tier-list surface over `marts.hero_current`. A real product, but a
  different one; it would force the uid-versus-mlid identity question that the
  evidence-only path avoids entirely.
- Hunting a separate counter-hero source. Closed: the ranking page renders its
  COUNTER HERO column from `sub_hero` on the endpoint already collected.
