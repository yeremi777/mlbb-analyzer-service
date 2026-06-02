# Dataset Contract

The Python service should own the static dataset after migration.

## Suggested Structure

```txt
data/
  heroes.json
  counters.json
  counters/
    miya.json
    balmond.json
    saber.json
    alice.json
    nana.json
    tigreal.json
    alucard.json
```

`data/counters.json` is an index:

```json
{
  "files": [
    "counters/alice.json",
    "counters/alucard.json",
    "counters/balmond.json",
    "counters/miya.json",
    "counters/nana.json",
    "counters/saber.json",
    "counters/tigreal.json"
  ]
}
```

## Counter Matchup

Static counter records do not include scores. Scores are runtime analyzer output.

```json
{
  "targetHeroId": "tigreal",
  "counterHeroId": "diggie",
  "reasons": [
    "Diggie reduces the impact of Tigreal's crowd-control engage."
  ],
  "counterTypes": ["anti-cc", "disengage", "teamfight"],
  "proof": [
    {
      "id": "diggie-time-journey-vs-tigreal-engage",
      "category": "skill-interaction",
      "priority": "primary",
      "impact": "high",
      "summary": "Tigreal's main threat is an AoE crowd-control engage, while Diggie's ultimate gives nearby allies cleanse and control immunity during the engage window.",
      "worksBestWhen": [
        "Diggie saves ultimate for Tigreal's real engage."
      ],
      "failureCases": [
        "Tigreal baits Diggie's ultimate before committing."
      ]
    }
  ],
  "patchVersion": "manual-proof-v1"
}
```

## Validation Rules

The service should reject datasets when:

- `heroes.json` is not an array.
- `counters.json` is not an index with valid file paths.
- A counter split file does not contain an array.
- A split file contains a `targetHeroId` that does not match the file name.
- `targetHeroId` or `counterHeroId` does not exist in `heroes.json`.
- A matchup is a self-counter.
- Duplicate target/counter pairs exist.
- `reasons` is empty or contains blank strings.
- `counterTypes` is empty or contains blank strings.
- `score` exists in static data.
- `proof.scoreHint` exists in static data.
- proof entries have invalid category, priority, or impact values.

## Allowed Proof Values

Categories:

- `skill-interaction`
- `crowd-control-counter`
- `damage-type-advantage`
- `item-power-spike`
- `mobility-advantage`
- `range-advantage`
- `kiting`
- `sustain-anti-sustain`
- `positioning-requirement`
- `cooldown-window`
- `vision-awareness`
- `teamfight-role-counter`
- `game-phase`
- `execution-difficulty`

Priorities:

- `primary`
- `secondary`
- `condition`

Impacts:

- `high`
- `medium`
- `low`

