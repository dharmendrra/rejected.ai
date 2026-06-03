// Package evaluators runs multiple independent evaluator personas over a
// completed interview. Each persona reasons from a distinct perspective
// (architect, manager, staff+, operator, ATS, communication, AI-native), so the
// report shows several viewpoints rather than a single black-box score.
package evaluators

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Persona describes one evaluator lens.
type Persona struct {
	Name string
	Lens string
}

// DefaultPersonas is the standard evaluator panel.
var DefaultPersonas = []Persona{
	{"Architect", "Judges system design, tradeoff analysis, and architectural reasoning depth."},
	{"Engineering Manager", "Judges leadership, ownership, collaboration, and delivery."},
	{"Staff+ Engineer", "Judges systems thinking, technical influence, and rigor at scale."},
	{"Practical Operator", "Judges operational maturity: reliability, observability, incident response."},
	{"ATS", "Strict rubric/keyword alignment to the job description (explicit terminology only)."},
	{"Communication", "Judges clarity, structure, and ability to explain complex ideas."},
	{"AI-Native", "Judges capability patterns: verification, governance, automation, context engineering — not tool names."},
}

// Service runs the persona panel.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs an evaluators Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

const personaSystem = `You are a panel of independent technical interviewers. Each persona evaluates the SAME
interview from its own perspective and reaches its own conclusions — do not make the
personas agree artificially. Ground every judgment in the evidence provided; cite the
relevant turns in your reasoning. Scores are 0..1. Respond with a single JSON object and
no prose.`

const personaUserTmpl = `Interview level: %s | type: %s
Competencies: %s

Final competency scores (cool/normal/hot lenses):
%s

Evidence ledger (by competency):
%s

For EACH of these personas: %s

Produce JSON:
{
  "personas": [
    {
      "persona": string,
      "overall_take": string,
      "endorsements": string[],
      "concerns": string[],
      "per_competency": [ { "competency": string, "score": number(0..1), "reasoning": string } ]
    }
  ]
}`

type personaResult struct {
	Personas []domain.PersonaView `json:"personas"`
}

// Evaluate runs the full persona panel in a single call.
func (s *Service) Evaluate(ctx context.Context, iv *domain.Interview, scores []domain.CompetencyScore, ledger []domain.EvidenceItem) ([]domain.PersonaView, error) {
	names := make([]string, len(DefaultPersonas))
	for i, p := range DefaultPersonas {
		names[i] = fmt.Sprintf("%s (%s)", p.Name, p.Lens)
	}

	user := fmt.Sprintf(personaUserTmpl,
		iv.Level, iv.Type,
		llm.MarshalCompact(iv.Competencies),
		renderScores(scores),
		renderLedgerByCompetency(ledger),
		llm.MarshalCompact(names),
	)

	var res personaResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, personaSystem, user, &res); err != nil {
		return nil, fmt.Errorf("persona evaluation: %w", err)
	}
	return res.Personas, nil
}

func renderScores(scores []domain.CompetencyScore) string {
	var b []byte
	for _, s := range scores {
		b = append(b, fmt.Sprintf("- %s: cool=%.2f normal=%.2f hot=%.2f (turns %v)\n",
			s.Competency, s.Cool, s.Normal, s.Hot, s.EvidenceTurns)...)
	}
	if len(b) == 0 {
		return "(none)"
	}
	return string(b)
}

func renderLedgerByCompetency(ledger []domain.EvidenceItem) string {
	byComp := map[string][]domain.EvidenceItem{}
	order := []string{}
	for _, it := range ledger {
		if _, ok := byComp[it.Competency]; !ok {
			order = append(order, it.Competency)
		}
		byComp[it.Competency] = append(byComp[it.Competency], it)
	}
	var b []byte
	for _, c := range order {
		b = append(b, fmt.Sprintf("\n## %s\n", c)...)
		for _, it := range byComp[c] {
			b = append(b, fmt.Sprintf("- turn %d | %s | strength %.2f | %q\n",
				it.Turn, it.Polarity, it.Strength, it.SupportingQuote)...)
		}
	}
	if len(b) == 0 {
		return "(no evidence)"
	}
	return string(b)
}
