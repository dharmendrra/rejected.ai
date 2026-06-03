package confidence

import (
	"context"
	"fmt"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Finalize rolls the per-turn confidence snapshots up into one final
// CompetencyScore per competency (the most recent snapshot wins, since each
// snapshot already reflects the full ledger). Results are persisted to
// competency_scores and returned.
func (s *Service) Finalize(ctx context.Context, interviewID bson.ObjectID) ([]domain.CompetencyScore, error) {
	cur, err := s.Store.Coll(store.CollConfidenceScores).Find(ctx,
		bson.D{{Key: "interview_id", Value: interviewID}},
		options.Find().SetSort(bson.D{{Key: "turn", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("load snapshots: %w", err)
	}
	var snaps []domain.ConfidenceSnapshot
	if err := cur.All(ctx, &snaps); err != nil {
		return nil, fmt.Errorf("decode snapshots: %w", err)
	}

	// Turn-ascending, so the last snapshot per competency is the latest belief.
	latest := map[string]domain.ConfidenceSnapshot{}
	order := []string{}
	for _, sn := range snaps {
		if _, ok := latest[sn.Competency]; !ok {
			order = append(order, sn.Competency)
		}
		latest[sn.Competency] = sn
	}

	now := time.Now().UTC()
	scores := make([]domain.CompetencyScore, 0, len(order))
	docs := make([]any, 0, len(order))
	for _, comp := range order {
		sn := latest[comp]
		score := domain.CompetencyScore{
			InterviewID:   interviewID,
			Competency:    comp,
			Confidence:    sn.Confidence,
			Cool:          sn.Cool,
			Normal:        sn.Normal,
			Hot:           sn.Hot,
			EvidenceTurns: sn.EvidenceTurns,
			Rationale:     sn.Rationale,
			CreatedAt:     now,
		}
		scores = append(scores, score)
		docs = append(docs, score)
	}

	if len(docs) > 0 {
		// Replace any prior finalization for idempotent re-runs.
		_, _ = s.Store.Coll(store.CollCompetencyScores).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
		if _, err := s.Store.Coll(store.CollCompetencyScores).InsertMany(ctx, docs); err != nil {
			return nil, fmt.Errorf("persist competency scores: %w", err)
		}
	}
	return scores, nil
}
