// Package store wraps the MongoDB connection and exposes typed access to the
// platform's collections. It uses the official driver v2
// (go.mongodb.org/mongo-driver/v2), connecting to a local mongod by default.
package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection names, one per domain concept in the spec.
const (
	CollJobDescriptions  = "job_descriptions"
	CollCandidateProfile = "candidate_profiles"
	CollInterviews       = "interviews"
	CollQuestions        = "questions"
	CollAnswers          = "answers"
	CollTranscripts      = "transcripts"
	CollVideoMetadata    = "video_metadata"
	CollCapabilityGraphs = "capability_graphs"
	CollConfidenceScores = "confidence_scores"
	CollCompetencyScores = "competency_scores"
	CollEvidenceLedger   = "evidence_ledger"
	CollSignals          = "signals"
	CollRiskAreas        = "risk_areas"
	CollRecommendations  = "recommendations"
	CollHistoricalTrends = "historical_trends"
	CollIdealResponses   = "ideal_responses"
	CollReportProgress   = "report_progress"
	CollCandidateCoaching = "candidate_coaching"
)

// Store holds the Mongo client and the active database handle.
type Store struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Connect dials MongoDB at uri and selects database dbName.
func Connect(ctx context.Context, uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return &Store{Client: client, DB: client.Database(dbName)}, nil
}

// Coll returns a handle to the named collection.
func (s *Store) Coll(name string) *mongo.Collection {
	return s.DB.Collection(name)
}

// Ping verifies the connection is alive (used by /healthz).
func (s *Store) Ping(ctx context.Context) error {
	return s.Client.Ping(ctx, nil)
}

// Disconnect closes the underlying client.
func (s *Store) Disconnect(ctx context.Context) error {
	return s.Client.Disconnect(ctx)
}
