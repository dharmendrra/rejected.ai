// Package interview orchestrates the live interview: it creates a session from a
// JD + resume, builds the capability graphs, generates questions dynamically
// from validation targets and confidence gaps, and runs each answer through
// evidence extraction and confidence re-scoring.
package interview

import (
	"context"
	"fmt"
	"time"

	"github.com/dharmendra/rejected.ai/internal/assumptions"
	"github.com/dharmendra/rejected.ai/internal/capability"
	"github.com/dharmendra/rejected.ai/internal/confidence"
	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service runs interviews end to end.
type Service struct {
	LLM         *llm.Provider
	Store       *store.Store
	Capability  *capability.Service
	Evidence    *evidence.Service
	Confidence  *confidence.Service
	Assumptions *assumptions.Service
}

// NewService wires the interview Service with its engine dependencies.
func NewService(provider *llm.Provider, st *store.Store, cap *capability.Service, ev *evidence.Service, conf *confidence.Service, asm *assumptions.Service) *Service {
	return &Service{LLM: provider, Store: st, Capability: cap, Evidence: ev, Confidence: conf, Assumptions: asm}
}

// CreateRequest is the input to start an interview.
type CreateRequest struct {
	JobDescriptionID   string `json:"job_description_id"`
	CandidateProfileID string `json:"candidate_profile_id"`
	Level              string `json:"level"`
	Type               string `json:"type"`
	DurationMin        int    `json:"duration_min"`
}

// CreateResult is returned when an interview starts.
type CreateResult struct {
	Interview *domain.Interview          `json:"interview"`
	Graphs    *domain.CapabilityGraphSet `json:"graphs"`
	Question  *domain.Turn               `json:"question"`
}

// CreateSession builds graphs, derives competencies, persists the interview, and
// asks the first question.
func (s *Service) CreateSession(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	jdID, err := bson.ObjectIDFromHex(req.JobDescriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid job_description_id: %w", err)
	}
	cpID, err := bson.ObjectIDFromHex(req.CandidateProfileID)
	if err != nil {
		return nil, fmt.Errorf("invalid candidate_profile_id: %w", err)
	}

	var jd domain.JobDescription
	if err := s.Store.Coll(store.CollJobDescriptions).FindOne(ctx, bson.D{{Key: "_id", Value: jdID}}).Decode(&jd); err != nil {
		return nil, fmt.Errorf("load job description: %w", err)
	}
	var cp domain.CandidateProfile
	if err := s.Store.Coll(store.CollCandidateProfile).FindOne(ctx, bson.D{{Key: "_id", Value: cpID}}).Decode(&cp); err != nil {
		return nil, fmt.Errorf("load candidate profile: %w", err)
	}

	if req.Level == "" {
		req.Level = "Senior Engineer"
	}
	if req.Type == "" {
		req.Type = "Mixed"
	}
	if req.DurationMin <= 0 {
		req.DurationMin = 20
	}

	graphs, err := s.Capability.Build(ctx, req.Level, req.Type, &jd, &cp)
	if err != nil {
		return nil, err
	}

	competencies := deriveCompetencies(graphs)
	now := time.Now().UTC()
	iv := domain.Interview{
		JobDescriptionID:   jdID,
		CandidateProfileID: cpID,
		Level:              req.Level,
		Type:               req.Type,
		DurationMin:        req.DurationMin,
		Status:             domain.StatusActive,
		Competencies:       competencies,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	res, err := s.Store.Coll(store.CollInterviews).InsertOne(ctx, iv)
	if err != nil {
		return nil, fmt.Errorf("persist interview: %w", err)
	}
	iv.ID = res.InsertedID.(bson.ObjectID)

	graphs.InterviewID = iv.ID
	graphs.CreatedAt = now
	gres, err := s.Store.Coll(store.CollCapabilityGraphs).InsertOne(ctx, graphs)
	if err != nil {
		return nil, fmt.Errorf("persist graphs: %w", err)
	}
	graphs.ID = gres.InsertedID.(bson.ObjectID)

	// First question.
	turn, err := s.generateQuestion(ctx, &iv, graphs, nil, 1)
	if err != nil {
		return nil, err
	}

	return &CreateResult{Interview: &iv, Graphs: graphs, Question: turn}, nil
}

// deriveCompetencies infers the competency set from the gap graph (validation
// targets first) plus high-weight target capabilities — never hardcoded.
func deriveCompetencies(g *domain.CapabilityGraphSet) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		key := normalize(name)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	for _, vt := range g.ValidationTargets {
		add(vt.Competency)
	}
	for _, t := range g.Target {
		if t.Weight >= 0.5 {
			add(t.Name)
		}
	}
	return out
}

// questionBudget converts the available time into an approximate number of
// questions (~3 minutes each), bounded for sanity.
func questionBudget(durationMin int) int {
	n := durationMin / 3
	if n < 3 {
		n = 3
	}
	if n > 12 {
		n = 12
	}
	return n
}
