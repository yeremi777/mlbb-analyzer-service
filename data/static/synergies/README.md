# Synergy Dataset

Hero **synergy** data — allies that *enable, protect, follow up, amplify, or complete* an
anchor hero's game plan (the positive counterpart of the `counters/` dataset).

The authoritative contract for this dataset is [`docs/07-synergy-proof-workflow.md`](../../../docs/07-synergy-proof-workflow.md).
This file is a short pointer to that spec.

## Structure

```txt
synergies.json            # index: list of per-anchor files
synergies/
  miya.json
  balmond.json
  ...
```

Each `synergies/<anchorHeroId>.json` is an array of `SynergyMatchup` entries for that anchor hero.

## Entry Shape (see docs/07 for the full contract)

```json
{
  "anchorHeroId": "tigreal",
  "synergyHeroId": "pharsa",
  "reasons": ["..."],
  "synergyTypes": ["engage-follow-up", "teamfight-combo"],
  "proof": [
    {
      "id": "pharsa-feathered-air-strike-with-tigreal-grouped-aoe",
      "category": "teamfight-combo",
      "priority": "primary",
      "impact": "high",
      "summary": "...",
      "worksBestWhen": ["..."],
      "failureCases": ["..."]
    }
  ]
}
```

- `anchorHeroId` / `synergyHeroId` must exist in `heroes.json`; they must differ; pairs are unique.
- No static runtime scoring fields.
- `proof.category` must be one of the allowed **synergy** categories in `docs/07-synergy-proof-workflow.md`
  (distinct from the counter `ProofCategory` enum).
- `priority` ∈ {`primary`, `secondary`, `condition`}; `impact` ∈ {`low`, `medium`, `high`}.

## Coverage

Curated for anchor hero IDs 1-20: `miya`, `balmond`, `saber`, `alice`, `nana`, `tigreal`,
`alucard`, `karina`, `akai`, `franco`, `bane`, `bruno`, `clint`, `rafaela`, `eudora`,
`zilong`, `fanny`, `layla`, `minotaur`, and `lolita` — 5 synergies each, manually curated
with public references used only to verify mechanics.

## Backend Wiring

The backend reads this data through the synergy loader, validation command, `GET /api/heroes/{id}/synergies`,
and the AI synergy analyze endpoints.
