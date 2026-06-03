package evidence

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// All returns the full evidence ledger for an interview, ordered by turn.
func (s *Service) All(ctx context.Context, interviewID bson.ObjectID) ([]domain.EvidenceItem, error) {
	cur, err := s.Store.Coll(store.CollEvidenceLedger).Find(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
	if err != nil {
		return nil, fmt.Errorf("find evidence: %w", err)
	}
	var items []domain.EvidenceItem
	if err := cur.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode evidence: %w", err)
	}
	return items, nil
}

// ApplyRevision updates an evidence item's strength and appends a revision
// record. Used by the confidence engine for retroactive re-scoring.
func (s *Service) ApplyRevision(ctx context.Context, id bson.ObjectID, newStrength float64, rev domain.Revision) error {
	_, err := s.Store.Coll(store.CollEvidenceLedger).UpdateByID(ctx, id, bson.D{
		{Key: "$set", Value: bson.D{{Key: "strength", Value: clamp01(newStrength)}}},
		{Key: "$push", Value: bson.D{{Key: "revisions", Value: rev}}},
	})
	if err != nil {
		return fmt.Errorf("apply evidence revision: %w", err)
	}
	return nil
}
