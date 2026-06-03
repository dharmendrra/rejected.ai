// Package report orchestrates the final assessment: it finalizes competency
// scores, runs the evaluator panel, derives strongest signals and risk areas,
// and produces the explainable hiring recommendation — persisting each piece and
// assembling them into one read model.
package report

import (
	"context"
	"fmt"
	"time"

	"github.com/dharmendra/rejected.ai/internal/confidence"
	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/evaluators"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/recommendation"
	"github.com/dharmendra/rejected.ai/internal/risk"
	"github.com/dharmendra/rejected.ai/internal/signals"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service generates and loads interview reports.
type Service struct {
	Store          *store.Store
	Evidence       *evidence.Service
	Confidence     *confidence.Service
	Evaluators     *evaluators.Service
	Signals        *signals.Service
	Risk           *risk.Service
	Recommendation *recommendation.Service
}

// NewService wires the report Service.
func NewService(st *store.Store, ev *evidence.Service, conf *confidence.Service, eval *evaluators.Service, sig *signals.Service, rk *risk.Service, rec *recommendation.Service) *Service {
	return &Service{Store: st, Evidence: ev, Confidence: conf, Evaluators: eval, Signals: sig, Risk: rk, Recommendation: rec}
}

// Report is the assembled final assessment read model.
type Report struct {
	Interview        *domain.Interview        `json:"interview"`
	CompetencyScores []domain.CompetencyScore `json:"competency_scores"`
	Signals          []domain.StrongestSignal `json:"signals"`
	Risks            []domain.RiskItem        `json:"risks"`
	Recommendation   *domain.Recommendation   `json:"recommendation"`
}

// Generate computes the full report, persists each component, and returns it.
func (s *Service) Generate(ctx context.Context, interviewID bson.ObjectID) (*Report, error) {
	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
		return nil, fmt.Errorf("load interview: %w", err)
	}
	var graphs domain.CapabilityGraphSet
	if err := s.Store.Coll(store.CollCapabilityGraphs).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&graphs); err != nil {
		return nil, fmt.Errorf("load graphs: %w", err)
	}

	scores, err := s.Confidence.Finalize(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	ledger, err := s.Evidence.All(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	personas, err := s.Evaluators.Evaluate(ctx, &iv, scores, ledger)
	if err != nil {
		return nil, err
	}

	sigs, err := s.Signals.Strongest(ctx, scores, ledger)
	if err != nil {
		return nil, err
	}
	risks, err := s.Risk.Assess(ctx, &graphs, scores)
	if err != nil {
		return nil, err
	}

	rec, err := s.Recommendation.Decide(ctx, &iv, scores, sigs, risks, personas)
	if err != nil {
		return nil, err
	}
	rec.Personas = personas
	rec.CreatedAt = time.Now().UTC()

	if err := s.persist(ctx, interviewID, sigs, risks, rec); err != nil {
		return nil, err
	}

	return &Report{
		Interview:        &iv,
		CompetencyScores: scores,
		Signals:          sigs,
		Risks:            risks,
		Recommendation:   rec,
	}, nil
}

// persist writes the signals, risk, and recommendation documents, replacing any
// prior copies so report regeneration is idempotent.
func (s *Service) persist(ctx context.Context, id bson.ObjectID, sigs []domain.StrongestSignal, risks []domain.RiskItem, rec *domain.Recommendation) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "interview_id", Value: id}}

	_, _ = s.Store.Coll(store.CollSignals).DeleteMany(ctx, filter)
	if _, err := s.Store.Coll(store.CollSignals).InsertOne(ctx, domain.SignalsDoc{InterviewID: id, Signals: sigs, CreatedAt: now}); err != nil {
		return fmt.Errorf("persist signals: %w", err)
	}

	_, _ = s.Store.Coll(store.CollRiskAreas).DeleteMany(ctx, filter)
	if _, err := s.Store.Coll(store.CollRiskAreas).InsertOne(ctx, domain.RiskDoc{InterviewID: id, Risks: risks, CreatedAt: now}); err != nil {
		return fmt.Errorf("persist risks: %w", err)
	}

	_, _ = s.Store.Coll(store.CollRecommendations).DeleteMany(ctx, filter)
	if _, err := s.Store.Coll(store.CollRecommendations).InsertOne(ctx, rec); err != nil {
		return fmt.Errorf("persist recommendation: %w", err)
	}
	return nil
}

// Load assembles a previously generated report from stored documents.
func (s *Service) Load(ctx context.Context, interviewID bson.ObjectID) (*Report, error) {
	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
		return nil, fmt.Errorf("load interview: %w", err)
	}

	rep := &Report{Interview: &iv}

	cur, err := s.Store.Coll(store.CollCompetencyScores).Find(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
	if err == nil {
		_ = cur.All(ctx, &rep.CompetencyScores)
	}

	var sigDoc domain.SignalsDoc
	if err := s.Store.Coll(store.CollSignals).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&sigDoc); err == nil {
		rep.Signals = sigDoc.Signals
	}

	var riskDoc domain.RiskDoc
	if err := s.Store.Coll(store.CollRiskAreas).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&riskDoc); err == nil {
		rep.Risks = riskDoc.Risks
	}

	var rec domain.Recommendation
	if err := s.Store.Coll(store.CollRecommendations).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&rec); err == nil {
		rep.Recommendation = &rec
	}

	return rep, nil
}
