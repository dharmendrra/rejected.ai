package signals

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Service derives the candidate's strongest demonstrated signals.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs a signals Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

const signalsSystem = `You identify a candidate's strongest demonstrated SIGNALS from interview evidence —
patterns like systems thinking, operational maturity, tradeoff analysis, leadership
communication, or governance thinking. Focus on capability patterns, not keywords. Each
signal must cite the turns that demonstrate it. Only include signals that are genuinely
well-supported. Respond with a single JSON object and no prose.`

const signalsUserTmpl = `Final competency scores:
%s

Evidence ledger (by competency):
%s

Produce JSON:
{ "signals": [ { "name": string, "description": string, "evidence_turns": number[] } ] }`

type signalsResult struct {
	Signals []domain.StrongestSignal `json:"signals"`
}

// Strongest returns the candidate's strongest demonstrated signals.
func (s *Service) Strongest(ctx context.Context, scores []domain.CompetencyScore, ledger []domain.EvidenceItem) ([]domain.StrongestSignal, error) {
	user := fmt.Sprintf(signalsUserTmpl, renderScoresList(scores), renderLedger(ledger))
	var res signalsResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, signalsSystem, user, &res); err != nil {
		return nil, fmt.Errorf("strongest signals: %w", err)
	}
	return res.Signals, nil
}

func renderScoresList(scores []domain.CompetencyScore) string {
	var b []byte
	for _, s := range scores {
		b = append(b, fmt.Sprintf("- %s: normal=%.2f (turns %v) — %s\n", s.Competency, s.Normal, s.EvidenceTurns, s.Rationale)...)
	}
	if len(b) == 0 {
		return "(none)"
	}
	return string(b)
}

func renderLedger(ledger []domain.EvidenceItem) string {
	var b []byte
	for _, it := range ledger {
		b = append(b, fmt.Sprintf("- [%s] turn %d | %s | strength %.2f | %q\n", it.Competency, it.Turn, it.Polarity, it.Strength, it.SupportingQuote)...)
	}
	if len(b) == 0 {
		return "(no evidence)"
	}
	return string(b)
}
