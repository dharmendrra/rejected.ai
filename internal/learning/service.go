package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Service computes and persists cross-interview trends for a candidate.
type Service struct {
	Store *store.Store
}

// NewService wires the learning Service.
func NewService(st *store.Store) *Service {
	return &Service{Store: st}
}

// ComputeForCandidate gathers every interview belonging to the candidate (in
// time order), loads each interview's finalized competency scores, builds each
// competency's trajectory, persists one trend document per (candidate,
// competency) — replacing any prior set — and returns the trends. Interviews
// without finalized scores contribute nothing.
func (s *Service) ComputeForCandidate(ctx context.Context, candidateID bson.ObjectID) ([]domain.HistoricalTrend, error) {
	cur, err := s.Store.Coll(store.CollInterviews).Find(ctx,
		bson.D{{Key: "candidate_profile_id", Value: candidateID}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("load interviews: %w", err)
	}
	var interviews []domain.Interview
	if err := cur.All(ctx, &interviews); err != nil {
		return nil, fmt.Errorf("decode interviews: %w", err)
	}

	scored := make([]ScoredInterview, 0, len(interviews))
	for _, iv := range interviews {
		scores, err := s.loadScores(ctx, iv.ID)
		if err != nil {
			return nil, err
		}
		if len(scores) == 0 {
			continue // no report generated for this interview yet
		}
		scored = append(scored, ScoredInterview{InterviewID: iv.ID, At: iv.CreatedAt, Scores: scores})
	}

	trends := BuildTrends(candidateID, scored, time.Now().UTC())

	if err := s.persist(ctx, candidateID, trends); err != nil {
		return nil, err
	}
	return trends, nil
}

// Load returns the persisted trends for a candidate, sorted by competency.
func (s *Service) Load(ctx context.Context, candidateID bson.ObjectID) ([]domain.HistoricalTrend, error) {
	cur, err := s.Store.Coll(store.CollHistoricalTrends).Find(ctx,
		bson.D{{Key: "candidate_id", Value: candidateID}},
		options.Find().SetSort(bson.D{{Key: "competency", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("load trends: %w", err)
	}
	var trends []domain.HistoricalTrend
	if err := cur.All(ctx, &trends); err != nil {
		return nil, fmt.Errorf("decode trends: %w", err)
	}
	return trends, nil
}

func (s *Service) loadScores(ctx context.Context, interviewID bson.ObjectID) ([]domain.CompetencyScore, error) {
	cur, err := s.Store.Coll(store.CollCompetencyScores).Find(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
	if err != nil {
		return nil, fmt.Errorf("load competency scores: %w", err)
	}
	var scores []domain.CompetencyScore
	if err := cur.All(ctx, &scores); err != nil {
		return nil, fmt.Errorf("decode competency scores: %w", err)
	}
	return scores, nil
}

// persist replaces the candidate's trend documents so recomputation is
// idempotent.
func (s *Service) persist(ctx context.Context, candidateID bson.ObjectID, trends []domain.HistoricalTrend) error {
	filter := bson.D{{Key: "candidate_id", Value: candidateID}}
	if _, err := s.Store.Coll(store.CollHistoricalTrends).DeleteMany(ctx, filter); err != nil {
		return fmt.Errorf("clear prior trends: %w", err)
	}
	if len(trends) == 0 {
		return nil
	}
	docs := make([]any, len(trends))
	for i := range trends {
		docs[i] = trends[i]
	}
	if _, err := s.Store.Coll(store.CollHistoricalTrends).InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("persist trends: %w", err)
	}
	return nil
}
