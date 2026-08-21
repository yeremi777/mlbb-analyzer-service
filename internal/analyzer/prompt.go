package analyzer

import (
	"bytes"
	"encoding/json"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

const scoringSystemInstruction = `You are scoring Mobile Legends hero counter recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each counter in the batch, return:
- counterHeroId: must match an id from the input
- score: 0-100 matchup strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: hard counter or very direct mechanic counter
- 85-94: strong and reliable counter
- 75-84: good counter with meaningful conditions
- 65-74: situational counter
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill interactions and direct crowd-control counters should score higher than generic damage or item-dependent counters.

Respond with JSON only in this shape:
{"recommendations":[{"counterHeroId":"...","score":0,"confidence":0}]}
Include every counterHeroId from the input exactly once.`

const detailSystemInstruction = `You are explaining one Mobile Legends hero counter matchup.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input.`

const synergyScoringSystemInstruction = `You are scoring Mobile Legends hero synergy recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each synergy in the batch, return:
- synergyHeroId: must match an id from the input
- score: 0-100 synergy strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: defining, near-mandatory pairing
- 85-94: strong and reliable synergy
- 75-84: good synergy with meaningful conditions
- 65-74: situational synergy
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill-combo and setup-into-payoff synergies should score higher than generic or purely situational pairings.

Respond with JSON only in this shape:
{"recommendations":[{"synergyHeroId":"...","score":0,"confidence":0}]}
Include every synergyHeroId from the input exactly once.`

const synergyDetailSystemInstruction = `You are explaining one Mobile Legends hero synergy pairing.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input.`

const detailRepairInstruction = `The previous JSON did not match the required detail response schema.

Return corrected JSON only with exactly these keys:
- score: integer 0-100
- confidence: integer 0-100
- summary: non-empty string
- strengths: non-empty array of strings from the provided reasons/proof
- conditions: array of strings from proof.worksBestWhen
- failureCases: array of strings from proof.failureCases
- evidenceIds: array of proof ids present in the input

Do not add matchup facts outside the provided dataset context.`

var languageInstructions = map[string]string{
	"en": "Write all user-visible explanation text (summary, strengths, conditions, failureCases) " +
		"in English. Keep every identifier exactly as given: counterHeroId, evidenceIds, and proof ids.",
	"id": "Write all user-visible explanation text (summary, strengths, conditions, failureCases) " +
		"in natural Indonesian (Bahasa Indonesia). " +
		"Do not translate or alter any identifier: keep counterHeroId, evidenceIds, and proof ids " +
		"exactly as given. Keep hero names as written. Only the explanatory prose should be Indonesian.",
}

func languageInstruction(language string) string {
	if s, ok := languageInstructions[language]; ok {
		return s
	}
	return languageInstructions["en"]
}

type heroContext struct {
	UID   string   `json:"uid"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	Lanes []string `json:"lanes"`
}

func heroCtx(h domain.Hero) heroContext {
	return heroContext{UID: h.UID, Name: h.Name, Roles: h.Roles, Lanes: h.Lanes}
}

type proofContext struct {
	ID            string    `json:"id"`
	Category      string    `json:"category"`
	Priority      string    `json:"priority"`
	Impact        string    `json:"impact"`
	Summary       string    `json:"summary"`
	WorksBestWhen *[]string `json:"worksBestWhen,omitempty"`
	FailureCases  *[]string `json:"failureCases,omitempty"`
}

func proofCtx(p domain.Proof, includeDetail bool) proofContext {
	ctx := proofContext{ID: p.ID, Category: p.Category, Priority: p.Priority, Impact: p.Impact, Summary: p.Summary}
	if includeDetail {
		wbw, fc := orEmpty(p.WorksBestWhen), orEmpty(p.FailureCases)
		ctx.WorksBestWhen, ctx.FailureCases = &wbw, &fc
	}
	return ctx
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func proofCtxs(proofs []domain.Proof, includeDetail bool) []proofContext {
	out := make([]proofContext, len(proofs))
	for i, p := range proofs {
		out[i] = proofCtx(p, includeDetail)
	}
	return out
}

// compactJSON matches the Python prompt encoding: compact separators, no
// HTML escaping, unicode kept as-is.
func compactJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

func userMessage(language string, payload any) Message {
	return Message{Role: "user", Content: languageInstruction(language) + "\n\nDataset context:\n" + compactJSON(payload)}
}

type counterMatchupContext struct {
	CounterHeroID string         `json:"counterHeroId"`
	Reasons       []string       `json:"reasons"`
	CounterTypes  []string       `json:"counterTypes"`
	Proof         []proofContext `json:"proof"`
	CounterHero   heroContext    `json:"counterHero"`
}

func buildScoringMessages(target domain.Hero, ms []domain.HeroMatchup, language string) []Message {
	ctxs := make([]counterMatchupContext, len(ms))
	for i, m := range ms {
		ctxs[i] = counterMatchupContext{
			CounterHeroID: m.Second.UID, Reasons: m.Reasons, CounterTypes: m.Types,
			Proof: proofCtxs(m.Proof, false), CounterHero: heroCtx(m.Second),
		}
	}
	payload := map[string]any{"targetHero": heroCtx(target), "matchups": ctxs, "outputLanguage": language}
	return []Message{{Role: "system", Content: scoringSystemInstruction}, userMessage(language, payload)}
}

func buildDetailMessages(target domain.Hero, m domain.HeroMatchup, language string) []Message {
	ctx := counterMatchupContext{
		CounterHeroID: m.Second.UID, Reasons: m.Reasons, CounterTypes: m.Types,
		Proof: proofCtxs(m.Proof, true), CounterHero: heroCtx(m.Second),
	}
	payload := map[string]any{"targetHero": heroCtx(target), "matchup": ctx, "outputLanguage": language}
	return []Message{{Role: "system", Content: detailSystemInstruction}, userMessage(language, payload)}
}

type synergyMatchupContext struct {
	SynergyHeroID string         `json:"synergyHeroId"`
	Reasons       []string       `json:"reasons"`
	SynergyTypes  []string       `json:"synergyTypes"`
	Proof         []proofContext `json:"proof"`
	SynergyHero   heroContext    `json:"synergyHero"`
}

func buildSynergyScoringMessages(anchor domain.Hero, ms []domain.HeroMatchup, language string) []Message {
	ctxs := make([]synergyMatchupContext, len(ms))
	for i, m := range ms {
		ctxs[i] = synergyMatchupContext{
			SynergyHeroID: m.Second.UID, Reasons: m.Reasons, SynergyTypes: m.Types,
			Proof: proofCtxs(m.Proof, false), SynergyHero: heroCtx(m.Second),
		}
	}
	payload := map[string]any{"anchorHero": heroCtx(anchor), "synergies": ctxs, "outputLanguage": language}
	return []Message{{Role: "system", Content: synergyScoringSystemInstruction}, userMessage(language, payload)}
}

func buildSynergyDetailMessages(anchor domain.Hero, m domain.HeroMatchup, language string) []Message {
	ctx := synergyMatchupContext{
		SynergyHeroID: m.Second.UID, Reasons: m.Reasons, SynergyTypes: m.Types,
		Proof: proofCtxs(m.Proof, true), SynergyHero: heroCtx(m.Second),
	}
	payload := map[string]any{"anchorHero": heroCtx(anchor), "synergy": ctx, "outputLanguage": language}
	return []Message{{Role: "system", Content: synergyDetailSystemInstruction}, userMessage(language, payload)}
}

func buildDetailRepairMessages(messages []Message, payload map[string]any, validationErr error, language string) []Message {
	return append(append([]Message{}, messages...),
		Message{Role: "assistant", Content: compactJSON(payload)},
		Message{Role: "user", Content: detailRepairInstruction + "\n\n" + languageInstruction(language) +
			"\n\nValidation error:\n" + validationErr.Error()},
	)
}
