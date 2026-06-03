// Package domain holds the core data types shared across engines. Keeping them
// in one dependency-free package avoids import cycles between documents,
// capability, interview, evidence, confidence, and evaluators.
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ─── Inputs ─────────────────────────────────────────────────────────────────

// JobDescription is the structured form of an uploaded/pasted JD.
type JobDescription struct {
	ID                        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Raw                       string        `bson:"raw" json:"raw,omitempty"`
	Title                     string        `bson:"title" json:"title"`
	Responsibilities          []string      `bson:"responsibilities" json:"responsibilities"`
	RequiredSkills            []string      `bson:"required_skills" json:"required_skills"`
	PreferredSkills           []string      `bson:"preferred_skills" json:"preferred_skills"`
	LeadershipExpectations    []string      `bson:"leadership_expectations" json:"leadership_expectations"`
	TechnicalExpectations     []string      `bson:"technical_expectations" json:"technical_expectations"`
	DomainExpectations        []string      `bson:"domain_expectations" json:"domain_expectations"`
	CommunicationExpectations []string      `bson:"communication_expectations" json:"communication_expectations"`
	CreatedAt                 time.Time     `bson:"created_at" json:"created_at"`
}

// CandidateProfile is the structured form of an uploaded resume.
type CandidateProfile struct {
	ID                    bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Raw                   string        `bson:"raw" json:"raw,omitempty"`
	Name                  string        `bson:"name" json:"name"`
	Experience            []string      `bson:"experience" json:"experience"`
	Technologies          []string      `bson:"technologies" json:"technologies"`
	ArchitectureEvidence  []string      `bson:"architecture_evidence" json:"architecture_evidence"`
	LeadershipEvidence    []string      `bson:"leadership_evidence" json:"leadership_evidence"`
	DeliveryEvidence      []string      `bson:"delivery_evidence" json:"delivery_evidence"`
	OperationalEvidence   []string      `bson:"operational_evidence" json:"operational_evidence"`
	DomainEvidence        []string      `bson:"domain_evidence" json:"domain_evidence"`
	AIEngineeringEvidence []string      `bson:"ai_engineering_evidence" json:"ai_engineering_evidence"`
	CreatedAt             time.Time     `bson:"created_at" json:"created_at"`
}

// ─── Capability graphs ──────────────────────────────────────────────────────

// Capability is a single demonstrated competency derived from a resume.
type Capability struct {
	Name     string   `bson:"name" json:"name"`
	Category string   `bson:"category" json:"category"`
	Evidence []string `bson:"evidence" json:"evidence"`
	Strength float64  `bson:"strength" json:"strength"` // 0..1 demonstrated strength
}

// TargetCapability is a competency the role requires/prefers, derived from a JD.
type TargetCapability struct {
	Name       string  `bson:"name" json:"name"`
	Category   string  `bson:"category" json:"category"`
	Importance string  `bson:"importance" json:"importance"` // "required" | "preferred"
	Weight     float64 `bson:"weight" json:"weight"`         // 0..1
}

// ValidationTarget is a competency the interview should focus on validating.
type ValidationTarget struct {
	Competency string  `bson:"competency" json:"competency"`
	Reason     string  `bson:"reason" json:"reason"`
	Priority   float64 `bson:"priority" json:"priority"` // 0..1
}

// CapabilityGraphSet bundles the three graphs for one interview.
type CapabilityGraphSet struct {
	ID                bson.ObjectID      `bson:"_id,omitempty" json:"id"`
	InterviewID       bson.ObjectID      `bson:"interview_id" json:"interview_id"`
	Candidate         []Capability       `bson:"candidate" json:"candidate"`
	Target            []TargetCapability `bson:"target" json:"target"`
	Strengths         []string           `bson:"strengths" json:"strengths"`
	Gaps              []string           `bson:"gaps" json:"gaps"`
	Unknowns          []string           `bson:"unknowns" json:"unknowns"`
	RiskAreas         []string           `bson:"risk_areas" json:"risk_areas"`
	ValidationTargets []ValidationTarget `bson:"validation_targets" json:"validation_targets"`
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
}

// ─── Interview session ──────────────────────────────────────────────────────

// Interview status values.
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

// Interview is the central session aggregate.
type Interview struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	JobDescriptionID   bson.ObjectID `bson:"job_description_id" json:"job_description_id"`
	CandidateProfileID bson.ObjectID `bson:"candidate_profile_id" json:"candidate_profile_id"`
	Level              string        `bson:"level" json:"level"`
	Type               string        `bson:"type" json:"type"`
	DurationMin        int           `bson:"duration_min" json:"duration_min"`
	Status             string        `bson:"status" json:"status"`
	Competencies       []string      `bson:"competencies" json:"competencies"` // inferred dynamically
	CreatedAt          time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time     `bson:"updated_at" json:"updated_at"`
}

// Turn kinds.
const (
	TurnQuestion = "question"
	TurnFollowup = "followup"
)

// Turn is one question (and its answer) in the conversation.
type Turn struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID        bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Turn               int           `bson:"turn" json:"turn"` // 1-based sequence
	Kind               string        `bson:"kind" json:"kind"`
	Question           string        `bson:"question" json:"question"`
	TargetCompetencies []string      `bson:"target_competencies" json:"target_competencies"`
	Answer             string        `bson:"answer" json:"answer"`
	Answered           bool          `bson:"answered" json:"answered"`
	// CompressionRatio is a measurable information-density signal for the answer.
	CompressionRatio float64 `bson:"compression_ratio" json:"compression_ratio"`

	// Phase 5 per-answer analysis.
	Assumptions       []string `bson:"assumptions,omitempty" json:"assumptions,omitempty"`
	ResponseType      string   `bson:"response_type,omitempty" json:"response_type,omitempty"` // answer | clarification | deflection
	ResponseReasoning string   `bson:"response_reasoning,omitempty" json:"response_reasoning,omitempty"`

	AskedAt    time.Time `bson:"asked_at" json:"asked_at"`
	AnsweredAt time.Time `bson:"answered_at" json:"answered_at"`
}

// Response types for clarification-vs-deflection classification.
const (
	ResponseAnswer        = "answer"
	ResponseClarification = "clarification"
	ResponseDeflection    = "deflection"
)

// ─── Evidence ledger ────────────────────────────────────────────────────────

// Evidence polarity.
const (
	PolarityPositive = "positive"
	PolarityNegative = "negative"
)

// Revision records a retroactive change to an evidence item's strength.
type Revision struct {
	AtTurn      int       `bson:"at_turn" json:"at_turn"` // the later turn that triggered the change
	OldStrength float64   `bson:"old_strength" json:"old_strength"`
	NewStrength float64   `bson:"new_strength" json:"new_strength"`
	Note        string    `bson:"note" json:"note"`
	At          time.Time `bson:"at" json:"at"`
}

// EvidenceItem is a single piece of evidence extracted from an answer.
type EvidenceItem struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID     bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Turn            int           `bson:"turn" json:"turn"`
	Competency      string        `bson:"competency" json:"competency"`
	Concepts        []string      `bson:"concepts" json:"concepts"` // concept-cluster links
	Polarity        string        `bson:"polarity" json:"polarity"`
	Strength        float64       `bson:"strength" json:"strength"` // 0..1 magnitude of the signal
	SupportingQuote string        `bson:"supporting_quote" json:"supporting_quote"`
	Interpretation  string        `bson:"interpretation" json:"interpretation"`
	Revisions       []Revision    `bson:"revisions,omitempty" json:"revisions"`
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
}

// ─── Confidence ─────────────────────────────────────────────────────────────

// ConfidenceSnapshot is the belief about a competency after a given turn.
// Snapshots accumulate across turns to form the score-evolution timeline.
type ConfidenceSnapshot struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID   bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Competency    string        `bson:"competency" json:"competency"`
	Turn          int           `bson:"turn" json:"turn"`
	Confidence    float64       `bson:"confidence" json:"confidence"` // normal-mode headline 0..1
	Cool          float64       `bson:"cool" json:"cool"`
	Normal        float64       `bson:"normal" json:"normal"`
	Hot           float64       `bson:"hot" json:"hot"`
	EvidenceCount int           `bson:"evidence_count" json:"evidence_count"`
	EvidenceTurns []int         `bson:"evidence_turns" json:"evidence_turns"`
	Rationale     string        `bson:"rationale" json:"rationale"`
	CreatedAt     time.Time     `bson:"created_at" json:"created_at"`
}

// ─── Final scores & report (Phases 6–7) ─────────────────────────────────────

// CompetencyScore is the final rolled-up assessment of one competency, with all
// three lenses and the evidence that supports it.
type CompetencyScore struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID   bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Competency    string        `bson:"competency" json:"competency"`
	Confidence    float64       `bson:"confidence" json:"confidence"`
	Cool          float64       `bson:"cool" json:"cool"`
	Normal        float64       `bson:"normal" json:"normal"`
	Hot           float64       `bson:"hot" json:"hot"`
	EvidenceTurns []int         `bson:"evidence_turns" json:"evidence_turns"`
	Rationale     string        `bson:"rationale" json:"rationale"`
	CreatedAt     time.Time     `bson:"created_at" json:"created_at"`
}

// PersonaCompetency is one persona's take on a single competency.
type PersonaCompetency struct {
	Competency string  `bson:"competency" json:"competency"`
	Score      float64 `bson:"score" json:"score"`
	Reasoning  string  `bson:"reasoning" json:"reasoning"`
}

// PersonaView is one evaluator persona's independent assessment.
type PersonaView struct {
	Persona       string              `bson:"persona" json:"persona"`
	OverallTake   string              `bson:"overall_take" json:"overall_take"`
	Endorsements  []string            `bson:"endorsements" json:"endorsements"`
	Concerns      []string            `bson:"concerns" json:"concerns"`
	PerCompetency []PersonaCompetency `bson:"per_competency" json:"per_competency"`
}

// StrongestSignal is a notable demonstrated strength with its supporting turns.
type StrongestSignal struct {
	Name          string `bson:"name" json:"name"`
	Description   string `bson:"description" json:"description"`
	EvidenceTurns []int  `bson:"evidence_turns" json:"evidence_turns"`
}

// SignalsDoc stores the strongest signals for an interview.
type SignalsDoc struct {
	ID          bson.ObjectID     `bson:"_id,omitempty" json:"id"`
	InterviewID bson.ObjectID     `bson:"interview_id" json:"interview_id"`
	Signals     []StrongestSignal `bson:"signals" json:"signals"`
	CreatedAt   time.Time         `bson:"created_at" json:"created_at"`
}

// Risk categories.
const (
	RiskMissing = "missing" // never demonstrated
	RiskWeak    = "weak"    // attempted but low confidence
	RiskJD      = "jd_risk" // required by role but insufficiently validated
)

// RiskItem is a single risk, categorized.
type RiskItem struct {
	Competency    string `bson:"competency" json:"competency"`
	Category      string `bson:"category" json:"category"`
	Severity      string `bson:"severity" json:"severity"` // low | medium | high
	Reason        string `bson:"reason" json:"reason"`
	EvidenceTurns []int  `bson:"evidence_turns" json:"evidence_turns"`
}

// RiskDoc stores all risk areas for an interview.
type RiskDoc struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Risks       []RiskItem    `bson:"risks" json:"risks"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
}

// Hiring decision values.
const (
	DecisionStrongHire    = "strong_hire"
	DecisionHire          = "hire"
	DecisionHireWithRisks = "hire_with_risks"
	DecisionBorderline    = "borderline"
	DecisionNoHire        = "no_hire"
)

// Citation ties a claim in the recommendation to specific evidence turns.
type Citation struct {
	Competency string `bson:"competency" json:"competency"`
	Turns      []int  `bson:"turns" json:"turns"`
	Note       string `bson:"note" json:"note"`
}

// Recommendation is the explainable hiring decision, with persona views embedded
// so every report shows the multi-evaluator perspective.
type Recommendation struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID     bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Decision        string        `bson:"decision" json:"decision"`
	ConfidenceLevel float64       `bson:"confidence_level" json:"confidence_level"`
	Reasoning       string        `bson:"reasoning" json:"reasoning"`
	Citations       []Citation    `bson:"citations" json:"citations"`
	Personas        []PersonaView `bson:"personas" json:"personas"`
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
}
