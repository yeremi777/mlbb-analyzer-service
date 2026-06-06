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

## Entry Shape (see docs/06 for the full contract)

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
- No `score`, no `proof.scoreHint`, no `patchVersion`.
- `proof.category` must be one of the allowed **synergy** categories in `docs/07-synergy-proof-workflow.md`
  (distinct from the counter `ProofCategory` enum).
- `priority` ∈ {`primary`, `secondary`, `condition`}; `impact` ∈ {`low`, `medium`, `high`}.

## Coverage

Curated for the first 10 anchor heroes (file order): `miya`, `balmond`, `saber`, `alice`,
`nana`, `tigreal`, `alucard`, `karina`, `akai`, `franco` — 5 synergies each, manually curated
with public MLBB guides used only to verify mechanics.

## Backend Wiring (follow-up, not yet implemented)

Data shape is ready; the backend does not read it yet. A follow-up task should add a
`SynergyProof` / `SynergyMatchup` schema (with the `docs/06` category enum, separate from the
counter enum), loader + validation coverage, and a `GET /api/heroes/{id}/synergies` endpoint.
