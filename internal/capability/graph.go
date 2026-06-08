// Package capability builds the candidate, target, and gap capability graphs
// that drive the interview. Everything is inferred by the LLM from the JD and
// resume — no hardcoded competencies, technologies, or categories.
package capability

import (
	"context"
	"fmt"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
)

// Service generates capability graph sets.
type Service struct {
	LLM *llm.Provider
}

// NewService constructs a capability Service.
func NewService(provider *llm.Provider) *Service {
	return &Service{LLM: provider}
}

const graphSystem = `You are a staff-level engineering interviewer building a capability map.
You compare a candidate's resume against a role's requirements to decide what the
interview must validate. Infer competencies dynamically from the inputs — never use a
fixed taxonomy. Be honest about gaps and unknowns; do not flatter the candidate.

Key definitions:
- "strengths": competencies the resume demonstrates AND the role values.
- "gaps": competencies the role requires but the resume does not evidence.
- "unknowns": competencies the role values where resume evidence is ambiguous/insufficient.
- "risk_areas": role-critical competencies where weak/missing evidence is a hiring risk.
- "validation_targets": what THIS interview should probe, highest priority first. Focus
  on unknowns and risks, not things already well-evidenced.

Respond with a single JSON object and no prose.`

const graphUserTmpl = `Build the capability graph set as JSON:
{
  "candidate": [ { "name": string, "category": string, "evidence": string[], "strength": number(0..1) } ],
  "target":    [ { "name": string, "category": string, "importance": "required"|"preferred", "weight": number(0..1) } ],
  "strengths": string[],
  "gaps": string[],
  "unknowns": string[],
  "risk_areas": string[],
  "validation_targets": [ { "competency": string, "reason": string, "priority": number(0..1) } ]
}

Interview level: %s
Interview type: %s

ROLE (structured job description):
%s

CANDIDATE (structured resume):
%s`

// Build generates the three graphs for a candidate/role pair at a given level
// and interview type. The returned set has no InterviewID; the caller assigns
// and persists it.
func (s *Service) Build(ctx context.Context, level, iType string, jd *domain.JobDescription, cp *domain.CandidateProfile) (*domain.CapabilityGraphSet, error) {
	// Temporarily clear Raw fields to avoid sending huge raw text blocks to the LLM prompt.
	origJDRaw := jd.Raw
	origCPRaw := cp.Raw
	jd.Raw = ""
	cp.Raw = ""
	defer func() {
		jd.Raw = origJDRaw
		cp.Raw = origCPRaw
	}()

	jdJSON := llm.MarshalCompact(jd)
	cpJSON := llm.MarshalCompact(cp)

	var set domain.CapabilityGraphSet
	user := fmt.Sprintf(graphUserTmpl, level, iType, jdJSON, cpJSON)
	if err := llm.CallJSON(ctx, s.LLM.Caller, graphSystem, user, &set); err != nil {
		return nil, fmt.Errorf("build capability graphs: %w", err)
	}
	return &set, nil
}
