// Package recommendation produces the explainable hiring decision. Every
// recommendation cites the evidence (competencies + turns) behind it — no
// black-box scoring.
package recommendation

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Service produces hiring recommendations.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs a recommendation Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

const recSystem = `You produce a hiring recommendation for an engineering candidate. Decision must be one of:
"strong_hire", "hire", "hire_with_risks", "borderline", "no_hire". Provide a calibrated
confidence_level (0..1) reflecting how much evidence supports the decision — low evidence
means low confidence, regardless of the score. Your reasoning MUST cite specific evidence:
every citation references a competency and the turns that support it. Weigh strengths,
risks, and the multiple evaluator perspectives. Respond with a single JSON object and no
prose.`

const recUserTmpl = `Interview level: %s | type: %s

Final competency scores:
%s

Strongest signals:
%s

Risk areas (missing/weak/jd_risk):
%s

Evaluator persona views:
%s

Produce JSON:
{
  "decision": "strong_hire"|"hire"|"hire_with_risks"|"borderline"|"no_hire",
  "confidence_level": number(0..1),
  "reasoning": string,
  "citations": [ { "competency": string, "turns": number[], "note": string } ]
}`

type recResult struct {
	Decision        string            `json:"decision"`
	ConfidenceLevel float64           `json:"confidence_level"`
	Reasoning       string            `json:"reasoning"`
	Citations       []domain.Citation `json:"citations"`
}

// Decide produces the recommendation (without persona embedding; the caller
// attaches persona views and persists).
func (s *Service) Decide(ctx context.Context, iv *domain.Interview, scores []domain.CompetencyScore, sigs []domain.StrongestSignal, risks []domain.RiskItem, personas []domain.PersonaView) (*domain.Recommendation, error) {
	user := fmt.Sprintf(recUserTmpl,
		iv.Level, iv.Type,
		llm.MarshalCompact(scores),
		llm.MarshalCompact(sigs),
		llm.MarshalCompact(risks),
		llm.MarshalCompact(personas),
	)
	var res recResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, recSystem, user, &res); err != nil {
		return nil, fmt.Errorf("recommendation: %w", err)
	}
	if !validDecision(res.Decision) {
		res.Decision = domain.DecisionBorderline
	}
	return &domain.Recommendation{
		InterviewID:     iv.ID,
		Decision:        res.Decision,
		ConfidenceLevel: res.ConfidenceLevel,
		Reasoning:       res.Reasoning,
		Citations:       res.Citations,
	}, nil
}

func validDecision(d string) bool {
	switch d {
	case domain.DecisionStrongHire, domain.DecisionHire, domain.DecisionHireWithRisks, domain.DecisionBorderline, domain.DecisionNoHire:
		return true
	}
	return false
}
