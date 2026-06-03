// Package risk categorizes hiring risks, clearly distinguishing evidence that is
// missing (never demonstrated), weak (attempted but low confidence), and JD-risk
// (required by the role but insufficiently validated).
package risk

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Service assesses risk areas.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs a risk Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

const riskSystem = `You categorize hiring RISK areas. Use exactly these categories:
- "missing": the candidate never demonstrated this competency at all.
- "weak": the candidate attempted it but confidence remains low.
- "jd_risk": the role REQUIRES this competency but it was insufficiently validated.
A single competency may appear under more than one category if warranted. Severity is
"low", "medium", or "high". Cite supporting turns where applicable. Be precise and do not
invent risks that the evidence and JD do not support. Respond with a single JSON object and
no prose.`

const riskUserTmpl = `Role requirements (target capabilities & validation targets):
%s

Final competency scores:
%s

Produce JSON:
{
  "risks": [
    { "competency": string, "category": "missing"|"weak"|"jd_risk",
      "severity": "low"|"medium"|"high", "reason": string, "evidence_turns": number[] }
  ]
}`

type riskResult struct {
	Risks []domain.RiskItem `json:"risks"`
}

// Assess categorizes the interview's risk areas.
func (s *Service) Assess(ctx context.Context, graphs *domain.CapabilityGraphSet, scores []domain.CompetencyScore) ([]domain.RiskItem, error) {
	user := fmt.Sprintf(riskUserTmpl,
		llm.MarshalCompact(map[string]any{"target": graphs.Target, "validation_targets": graphs.ValidationTargets, "gaps": graphs.Gaps, "risk_areas": graphs.RiskAreas}),
		renderScores(scores),
	)
	var res riskResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, riskSystem, user, &res); err != nil {
		return nil, fmt.Errorf("assess risk: %w", err)
	}
	// Normalize categories defensively.
	for i := range res.Risks {
		switch res.Risks[i].Category {
		case domain.RiskMissing, domain.RiskWeak, domain.RiskJD:
		default:
			res.Risks[i].Category = domain.RiskWeak
		}
	}
	return res.Risks, nil
}

func renderScores(scores []domain.CompetencyScore) string {
	var b []byte
	for _, s := range scores {
		b = append(b, fmt.Sprintf("- %s: normal=%.2f cool=%.2f hot=%.2f (turns %v)\n", s.Competency, s.Normal, s.Cool, s.Hot, s.EvidenceTurns)...)
	}
	if len(b) == 0 {
		return "(no competencies scored — likely no substantive answers)"
	}
	return string(b)
}
