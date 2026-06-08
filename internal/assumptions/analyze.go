// Package assumptions analyzes a single answer for the assumptions the candidate
// made and whether the response is a genuine answer, a valid clarification, or a
// deflection. This feeds the follow-up engine (clarification is encouraged;
// repeated deflection is a negative signal) and the interview replay.
package assumptions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Service performs per-answer assumption and response-type analysis.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs an assumptions Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

// Analysis is the result of analyzing one answer.
type Analysis struct {
	Assumptions  []string `json:"assumptions"`
	ResponseType string   `json:"response_type"`
	Reasoning    string   `json:"reasoning"`
}

// UnmarshalJSON implements a custom json.Unmarshaler to robustly handle cases
// where the LLM might wrap assumptions inside nested arrays (e.g. [["a", "b"]]).
func (a *Analysis) UnmarshalJSON(data []byte) error {
	type Alias Analysis
	aux := struct {
		Assumptions any `json:"assumptions"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Assumptions == nil {
		a.Assumptions = nil
		return nil
	}

	switch val := aux.Assumptions.(type) {
	case []any:
		var assumptions []string
		for _, item := range val {
			switch subVal := item.(type) {
			case string:
				assumptions = append(assumptions, subVal)
			case []any:
				for _, subSubVal := range subVal {
					if str, ok := subSubVal.(string); ok {
						assumptions = append(assumptions, str)
					}
				}
			}
		}
		a.Assumptions = assumptions
	case string:
		a.Assumptions = []string{val}
	}
	return nil
}

const analyzeSystem = `You analyze a single interview answer. Two jobs:

1) List the ASSUMPTIONS the candidate made (about scope, ownership, requirements, the
   environment, available primitives, etc.). Empty if none.

2) Classify the response as exactly one of:
   - "answer": the candidate substantively addressed the question.
   - "clarification": the candidate (validly) asked to clarify scope/requirements/ownership
     before answering. This is GOOD behavior, not avoidance.
   - "deflection": the candidate avoided answering, stalled, or repeatedly delayed without
     a legitimate clarification need.

Be fair: do not label concise answers as deflection. Distinguish a real clarifying question
from avoidance. Respond with a single JSON object and no prose.`

const analyzeUserTmpl = `CONVERSATION SO FAR:
%s

CURRENT QUESTION (turn %d):
%s

CANDIDATE ANSWER:
"""
%s
"""

Respond as JSON:
{ "assumptions": string[], "response_type": "answer"|"clarification"|"deflection", "reasoning": string }`

// Analyze runs the analysis for one answered turn.
func (s *Service) Analyze(ctx context.Context, turn *domain.Turn, memory string) (*Analysis, error) {
	user := fmt.Sprintf(analyzeUserTmpl, memory, turn.Turn, turn.Question, turn.Answer)
	var a Analysis
	if err := llm.CallJSON(ctx, s.LLM.Caller, analyzeSystem, user, &a); err != nil {
		return nil, fmt.Errorf("analyze answer: %w", err)
	}
	switch a.ResponseType {
	case domain.ResponseAnswer, domain.ResponseClarification, domain.ResponseDeflection:
	default:
		a.ResponseType = domain.ResponseAnswer
	}
	return &a, nil
}
