package documents

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

// Service structures uploaded documents into domain types and persists them.
type Service struct {
	LLM   *llm.Provider
	Store *store.Store
}

// NewService constructs an ingestion Service.
func NewService(provider *llm.Provider, st *store.Store) *Service {
	return &Service{LLM: provider, Store: st}
}

const jdSystem = `You are an expert technical recruiter extracting structured data from a job description.
Do not invent requirements that are not present. If a field has no content, return an empty array.
Infer nothing about the candidate; only describe what the role asks for.
Respond with a single JSON object and no prose.`

const jdUserTmpl = `Extract the following fields from this job description as JSON:
{
  "title": string,
  "responsibilities": string[],
  "required_skills": string[],
  "preferred_skills": string[],
  "leadership_expectations": string[],
  "technical_expectations": string[],
  "domain_expectations": string[],
  "communication_expectations": string[]
}

JOB DESCRIPTION:
"""
%s
"""`

// IngestJD structures raw JD text and persists it.
func (s *Service) IngestJD(ctx context.Context, raw string) (*domain.JobDescription, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("job description text is empty")
	}

	var jd domain.JobDescription
	if err := llm.CallJSON(ctx, s.LLM.Caller, jdSystem, fmt.Sprintf(jdUserTmpl, raw), &jd); err != nil {
		return nil, fmt.Errorf("structure jd: %w", err)
	}
	jd.Raw = raw
	jd.CreatedAt = time.Now().UTC()

	res, err := s.Store.Coll(store.CollJobDescriptions).InsertOne(ctx, jd)
	if err != nil {
		return nil, fmt.Errorf("persist jd: %w", err)
	}
	jd.ID = res.InsertedID.(bson.ObjectID)
	return &jd, nil
}

const resumeSystem = `You are an expert technical recruiter extracting structured data from a candidate resume.
Extract only what the resume actually states. Do not invent or embellish.
Each "*_evidence" array should contain concrete evidence phrases drawn from the resume.
Respond with a single JSON object and no prose.`

const resumeUserTmpl = `Extract the following fields from this resume as JSON:
{
  "name": string,
  "experience": string[],
  "technologies": string[],
  "architecture_evidence": string[],
  "leadership_evidence": string[],
  "delivery_evidence": string[],
  "operational_evidence": string[],
  "domain_evidence": string[],
  "ai_engineering_evidence": string[]
}

RESUME:
"""
%s
"""`

// IngestResume structures raw resume text and persists it.
func (s *Service) IngestResume(ctx context.Context, raw string) (*domain.CandidateProfile, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("resume text is empty")
	}

	var cp domain.CandidateProfile
	if err := llm.CallJSON(ctx, s.LLM.Caller, resumeSystem, fmt.Sprintf(resumeUserTmpl, raw), &cp); err != nil {
		return nil, fmt.Errorf("structure resume: %w", err)
	}
	cp.Raw = raw
	cp.CreatedAt = time.Now().UTC()

	res, err := s.Store.Coll(store.CollCandidateProfile).InsertOne(ctx, cp)
	if err != nil {
		return nil, fmt.Errorf("persist resume: %w", err)
	}
	cp.ID = res.InsertedID.(bson.ObjectID)
	return &cp, nil
}
