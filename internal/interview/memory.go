package interview

import (
	"context"
	"fmt"
	"strings"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// turns returns all turns for an interview ordered by sequence.
func (s *Service) turns(ctx context.Context, interviewID bson.ObjectID) ([]domain.Turn, error) {
	cur, err := s.Store.Coll(store.CollQuestions).Find(ctx,
		bson.D{{Key: "interview_id", Value: interviewID}},
		options.Find().SetSort(bson.D{{Key: "turn", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find turns: %w", err)
	}
	var ts []domain.Turn
	if err := cur.All(ctx, &ts); err != nil {
		return nil, fmt.Errorf("decode turns: %w", err)
	}
	return ts, nil
}

// buildMemory renders the conversation so far as plain text for prompts, so
// later questions and evaluations can use earlier answers as context.
func buildMemory(ts []domain.Turn) string {
	if len(ts) == 0 {
		return "(no prior exchanges)"
	}
	var b strings.Builder
	for _, t := range ts {
		fmt.Fprintf(&b, "Turn %d (%s) Q: %s\n", t.Turn, t.Kind, t.Question)
		if t.Answered {
			fmt.Fprintf(&b, "Turn %d A: %s\n", t.Turn, t.Answer)
		}
	}
	return strings.TrimSpace(b.String())
}
