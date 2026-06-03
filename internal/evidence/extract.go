// Package evidence extracts structured evidence from candidate answers and
// maintains the evidence ledger. Evaluation is concept-based, not keyword-based:
// each evidence item links to the concepts it demonstrates (e.g. "duplicate
// handling" -> idempotency, retry-safety, exactly-once), so concise shorthand is
// credited for the understanding it implies.
package evidence

import (
	"context"
	"fmt"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service extracts and stores evidence.
type Service struct {
	LLM   *llm.Provider
	Store *store.Store
}

// NewService constructs an evidence Service.
func NewService(provider *llm.Provider, st *store.Store) *Service {
	return &Service{LLM: provider, Store: st}
}

const extractSystem = `You are an evidence extractor for a technical interview. Given a question and the
candidate's answer, you identify discrete pieces of evidence about the candidate's
competencies.

Principles:
- Evaluate CONCEPTS, not vocabulary. If a candidate uses dense shorthand (e.g. "duplicate
  handling"), map it to the concepts it implies (idempotency, retry-safety, exactly-once)
  and credit the understanding. Do NOT penalize brevity here.
- Each evidence item ties to ONE competency (use the interview's competency names where
  they fit; otherwise name the competency precisely).
- "polarity": "positive" if it supports competence, "negative" if it reveals a gap or
  misconception.
- "strength" (0..1) is how strong/clear this single signal is on its own. Ambiguous
  shorthand should get a MODEST strength now; it can be revised upward later if a future
  answer clarifies it.
- "supporting_quote" must be an exact span from the answer.
- If the answer contains no real signal, return an empty array.

Respond with a single JSON object and no prose.`

const extractUserTmpl = `Interview competencies under assessment: %s

CONVERSATION SO FAR (most recent last):
%s

CURRENT QUESTION (turn %d, targets: %s):
%s

CANDIDATE ANSWER:
"""
%s
"""

Extract evidence as JSON:
{
  "evidence": [
    {
      "competency": string,
      "concepts": string[],
      "polarity": "positive"|"negative",
      "strength": number(0..1),
      "supporting_quote": string,
      "interpretation": string
    }
  ]
}`

type extractResult struct {
	Evidence []struct {
		Competency      string   `json:"competency"`
		Concepts        []string `json:"concepts"`
		Polarity        string   `json:"polarity"`
		Strength        float64  `json:"strength"`
		SupportingQuote string   `json:"supporting_quote"`
		Interpretation  string   `json:"interpretation"`
	} `json:"evidence"`
}

// Extract pulls evidence items from a single answered turn and persists them.
func (s *Service) Extract(ctx context.Context, iv *domain.Interview, turn *domain.Turn, memory string) ([]domain.EvidenceItem, error) {
	user := fmt.Sprintf(extractUserTmpl,
		llm.MarshalCompact(iv.Competencies),
		memory,
		turn.Turn,
		llm.MarshalCompact(turn.TargetCompetencies),
		turn.Question,
		turn.Answer,
	)

	var res extractResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, extractSystem, user, &res); err != nil {
		return nil, fmt.Errorf("extract evidence: %w", err)
	}

	now := time.Now().UTC()
	items := make([]domain.EvidenceItem, 0, len(res.Evidence))
	docs := make([]any, 0, len(res.Evidence))
	for _, e := range res.Evidence {
		polarity := domain.PolarityPositive
		if e.Polarity == domain.PolarityNegative {
			polarity = domain.PolarityNegative
		}
		item := domain.EvidenceItem{
			InterviewID:     iv.ID,
			Turn:            turn.Turn,
			Competency:      e.Competency,
			Concepts:        e.Concepts,
			Polarity:        polarity,
			Strength:        clamp01(e.Strength),
			SupportingQuote: e.SupportingQuote,
			Interpretation:  e.Interpretation,
			CreatedAt:       now,
		}
		items = append(items, item)
		docs = append(docs, item)
	}

	if len(docs) > 0 {
		res, err := s.Store.Coll(store.CollEvidenceLedger).InsertMany(ctx, docs)
		if err != nil {
			return nil, fmt.Errorf("persist evidence: %w", err)
		}
		for i, id := range res.InsertedIDs {
			items[i].ID = id.(bson.ObjectID)
		}
	}
	return items, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
