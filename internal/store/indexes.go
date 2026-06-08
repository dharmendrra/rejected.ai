package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// EnsureIndexes creates the indexes the platform relies on. It is idempotent:
// Mongo ignores index creation when an equivalent index already exists.
//
// The dominant access pattern is "everything for one interview", so most
// collections are indexed by interview_id; confidence/competency snapshots add
// the dimensions needed for score-evolution replay.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	specs := map[string][]mongo.IndexModel{
		CollInterviews: {
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		CollQuestions: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "turn", Value: 1}}},
		},
		CollAnswers: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "turn", Value: 1}}},
		},
		CollEvidenceLedger: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "competency", Value: 1}}},
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "turn", Value: 1}}},
		},
		CollConfidenceScores: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "competency", Value: 1}, {Key: "turn", Value: 1}}},
		},
		CollCompetencyScores: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}, {Key: "competency", Value: 1}}},
		},
		CollCapabilityGraphs: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollSignals: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollRiskAreas: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollRecommendations: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollTranscripts: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollVideoMetadata: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollHistoricalTrends: {
			{Keys: bson.D{{Key: "candidate_id", Value: 1}, {Key: "competency", Value: 1}}},
		},
		CollIdealResponses: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
		CollReportProgress: {
			{Keys: bson.D{{Key: "interview_id", Value: 1}}},
		},
	}

	for coll, models := range specs {
		if _, err := s.Coll(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("ensure indexes on %s: %w", coll, err)
		}
	}
	return nil
}
