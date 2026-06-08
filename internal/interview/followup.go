package interview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// followupThreshold: a targeted competency below this normal-confidence after
	// an answer warrants a clarifying follow-up before moving on.
	followupThreshold = 0.45
	// maxFollowupsPerComp caps follow-ups per competency so we seek clarification
	// without badgering.
	maxFollowupsPerComp = 1
)

// chooseFollowupCompetency decides whether the just-answered turn warrants a
// follow-up, returning the competency to probe. It seeks clarification before
// scoring down a low-confidence validation target, and also follows up on a
// deflection. Returns ok=false when no follow-up is needed.
func chooseFollowupCompetency(current *domain.Turn, latest map[string]domain.ConfidenceSnapshot, ts []domain.Turn) (string, bool) {
	deflected := current.ResponseType == domain.ResponseDeflection

	for _, comp := range current.TargetCompetencies {
		if followupsForComp(ts, comp) >= maxFollowupsPerComp {
			continue
		}
		snap, ok := latest[comp]
		lowConfidence := ok && snap.Normal < followupThreshold
		if lowConfidence || deflected {
			return comp, true
		}
	}
	return "", false
}

// followupsForComp counts prior follow-up turns aimed at a competency.
func followupsForComp(ts []domain.Turn, comp string) int {
	n := 0
	for _, t := range ts {
		if t.Kind != domain.TurnFollowup {
			continue
		}
		for _, c := range t.TargetCompetencies {
			if normalize(c) == normalize(comp) {
				n++
				break
			}
		}
	}
	return n
}

const followupSystem = `You are an interviewer asking a focused FOLLOW-UP. The candidate's previous answer left
a key competency under-validated, or was a clarification/deflection. Ask ONE follow-up that
gives the candidate a fair chance to demonstrate the competency — seek clarification first,
do not penalize brevity. Build directly on what they just said. Respond with a single JSON
object and no prose.`

const followupUserTmpl = `Competency to validate: %s
Interview level: %s | type: %s | rigor/difficulty: %d%%

Conversation so far:
%s

Generate the follow-up as JSON:
{ "question": string, "target_competencies": string[], "rationale": string }`

// generateFollowup creates and persists a follow-up turn for a competency.
func (s *Service) generateFollowup(ctx context.Context, iv *domain.Interview, comp string, turnNum int) (*domain.Turn, error) {
	ts, err := s.turns(ctx, iv.ID)
	if err != nil {
		return nil, err
	}
	user := fmt.Sprintf(followupUserTmpl, comp, iv.Level, iv.Type, iv.RigorPercent, buildMemory(ts))

	var qr questionResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, followupSystem, user, &qr); err != nil {
		return nil, fmt.Errorf("generate followup: %w", err)
	}
	targets := qr.TargetCompetencies
	if len(targets) == 0 {
		targets = []string{comp}
	}

	turn := domain.Turn{
		InterviewID:        iv.ID,
		Turn:               turnNum,
		Kind:               domain.TurnFollowup,
		Question:           strings.TrimSpace(qr.Question),
		TargetCompetencies: targets,
		AskedAt:            time.Now().UTC(),
	}
	res, err := s.Store.Coll(store.CollQuestions).InsertOne(ctx, turn)
	if err != nil {
		return nil, fmt.Errorf("persist followup: %w", err)
	}
	turn.ID = res.InsertedID.(bson.ObjectID)
	return &turn, nil
}
