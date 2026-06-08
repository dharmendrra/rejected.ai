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

// GraphStatus values for background capability graph build.
const (
	GraphStatusBuilding = "building"
	GraphStatusReady    = "ready"
	GraphStatusFailed   = "failed"
)

// Interview is the central session aggregate.
type Interview struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	JobDescriptionID   bson.ObjectID `bson:"job_description_id" json:"job_description_id"`
	CandidateProfileID bson.ObjectID `bson:"candidate_profile_id" json:"candidate_profile_id"`
	Level              string        `bson:"level" json:"level"`
	Type               string        `bson:"type" json:"type"`
	DurationMin        int           `bson:"duration_min" json:"duration_min"`
	RigorPercent       int           `bson:"rigor_percent" json:"rigor_percent"`
	Status             string        `bson:"status" json:"status"`
	GraphStatus        string        `bson:"graph_status,omitempty" json:"graph_status,omitempty"` // building | ready | failed
	Competencies       []string      `bson:"competencies" json:"competencies"`                     // inferred dynamically
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

// ─── Audio (Phase 9) ────────────────────────────────────────────────────────

// Transcript sources.
const (
	TranscriptWhisper  = "whisper"
	TranscriptProvided = "provided"
)

// FillerStat is the count of one filler word/phrase in an answer.
type FillerStat struct {
	Word  string `bson:"word" json:"word"`
	Count int    `bson:"count" json:"count"`
}

// Transcript holds the transcript of one answer plus MEASURABLE-ONLY audio
// signals. It deliberately avoids any inferred trait (honesty, intelligence,
// personality) — only directly countable quantities are stored.
type Transcript struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Turn        int           `bson:"turn" json:"turn"`
	Source      string        `bson:"source" json:"source"`
	Text        string        `bson:"text" json:"text"`
	DurationSec float64       `bson:"duration_sec" json:"duration_sec"`
	WordCount   int           `bson:"word_count" json:"word_count"`
	WPM         float64       `bson:"wpm" json:"wpm"` // speaking pace, words/min
	FillerTotal int           `bson:"filler_total" json:"filler_total"`
	FillerRate  float64       `bson:"filler_rate" json:"filler_rate"` // fillers per 100 words
	Fillers     []FillerStat  `bson:"fillers" json:"fillers"`
	LatencyMs   int           `bson:"latency_ms" json:"latency_ms"` // response latency, if provided
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
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

// ─── Video (Phase 10) ────────────────────────────────────────────────────────
//
// Video yields MEASURABLE-ONLY signals about engagement, attention, participation,
// and timing — derived from raw per-frame observations. It deliberately NEVER
// infers honesty, intelligence, personality, mood, or any unmeasurable trait;
// every stored field is a count, a percentage of counted frames, or a duration.

// VideoMetadata sources.
const (
	VideoDetector = "detector" // frame metrics produced from an uploaded clip
	VideoProvided = "provided" // frame metrics supplied directly by the caller
)

// FrameMetrics are the raw per-turn frame counts a detector (or the caller)
// reports for one answer's video. These are the only inputs to video analysis:
// nothing is inferred, everything is counted. A frame is "analyzed" if the
// detector inspected it at all.
type FrameMetrics struct {
	FramesAnalyzed     int     `bson:"frames_analyzed" json:"frames_analyzed"`
	FramesFacePresent  int     `bson:"frames_face_present" json:"frames_face_present"`
	FramesGazeOnScreen int     `bson:"frames_gaze_on_screen" json:"frames_gaze_on_screen"`
	FramesMultiFace    int     `bson:"frames_multi_face" json:"frames_multi_face"` // >1 face detected
	OnCameraSec        float64 `bson:"on_camera_sec" json:"on_camera_sec"`         // seconds with the candidate on camera
	DurationSec        float64 `bson:"duration_sec" json:"duration_sec"`           // total clip length
}

// VideoMetadata holds the measurable video signals for one answer turn. The Pct
// fields are shares of FramesAnalyzed (0 when no frames were analyzed), so they
// describe what was observed, never an inferred quality.
type VideoMetadata struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID     bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Turn            int           `bson:"turn" json:"turn"`
	Source          string        `bson:"source" json:"source"`
	FramesAnalyzed  int           `bson:"frames_analyzed" json:"frames_analyzed"`
	FacePresentPct  float64       `bson:"face_present_pct" json:"face_present_pct"`     // engagement: frames with a face
	GazeOnScreenPct float64       `bson:"gaze_on_screen_pct" json:"gaze_on_screen_pct"` // attention: frames looking at screen
	OnCameraPct     float64       `bson:"on_camera_pct" json:"on_camera_pct"`           // participation: on-camera share of duration
	MultiFacePct    float64       `bson:"multi_face_pct" json:"multi_face_pct"`         // frames with more than one face
	DurationSec     float64       `bson:"duration_sec" json:"duration_sec"`             // timing
	LatencyMs       int           `bson:"latency_ms" json:"latency_ms"`                 // response latency, if provided
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
}

// ─── Cross-interview learning (Phase 11) ─────────────────────────────────────
//
// Trends track how a candidate's measured competency scores move across their
// own interviews over time. They are deterministic and explainable: every value
// is derived from stored CompetencyScores, ordered by interview time. No trait
// is inferred and no LLM is involved — this is arithmetic over prior results.

// Trend directions.
const (
	TrendNew       = "new"       // only one interview so far — no trajectory yet
	TrendImproving = "improving" // latest score is meaningfully above the first
	TrendDeclining = "declining" // latest score is meaningfully below the first
	TrendStable    = "stable"    // change within noise
)

// TrendPoint is one competency measurement at one interview, in time order.
type TrendPoint struct {
	InterviewID bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Normal      float64       `bson:"normal" json:"normal"`         // balanced-lens score (the headline metric)
	Confidence  float64       `bson:"confidence" json:"confidence"` // evidence confidence behind the score
	At          time.Time     `bson:"at" json:"at"`                 // interview time
}

// HistoricalTrend is one candidate's trajectory on one competency across all of
// their interviews. There is one document per (candidate_id, competency).
type HistoricalTrend struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	CandidateID bson.ObjectID `bson:"candidate_id" json:"candidate_id"`
	Competency  string        `bson:"competency" json:"competency"`
	Points      []TrendPoint  `bson:"points" json:"points"`         // oldest-first
	Interviews  int           `bson:"interviews" json:"interviews"` // number of points
	First       float64       `bson:"first" json:"first"`           // earliest Normal score
	Latest      float64       `bson:"latest" json:"latest"`         // most recent Normal score
	Delta       float64       `bson:"delta" json:"delta"`           // latest - first
	Direction   string        `bson:"direction" json:"direction"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
}

// IdealResponse specifies what a candidate should have answered to achieve a >85% score.
type IdealResponse struct {
	Question     string   `bson:"question" json:"question"`
	Competency   string   `bson:"competency" json:"competency"`
	KeyPoints    []string `bson:"key_points" json:"key_points"`
	SampleAnswer string   `bson:"sample_answer" json:"sample_answer"`
}

// IdealResponsesDoc is the persisted collection of ideal responses for an interview.
type IdealResponsesDoc struct {
	ID          bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	InterviewID bson.ObjectID   `bson:"interview_id" json:"interview_id"`
	Responses   []IdealResponse `bson:"responses" json:"responses"`
	CreatedAt   time.Time       `bson:"created_at" json:"created_at"`
}

// ReportStep represents one step in the report generation process.
type ReportStep struct {
	Name   string `bson:"name" json:"name"`
	Status string `bson:"status" json:"status"` // "pending", "running", "completed"
}

// ReportProgress tracks the status of asynchronous report generation.
type ReportProgress struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	InterviewID    bson.ObjectID `bson:"interview_id" json:"interview_id"`
	Status         string        `bson:"status" json:"status"` // "generating", "completed", "failed"
	TotalSteps     int           `bson:"total_steps" json:"total_steps"`
	CompletedSteps int           `bson:"completed_steps" json:"completed_steps"`
	CurrentStep    string        `bson:"current_step" json:"current_step"`
	Steps          []ReportStep  `bson:"steps" json:"steps"`
	Error          string        `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt      time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at" json:"updated_at"`
}

// CoachingItem is an actionable feedback entry to help candidates improve.
type CoachingItem struct {
	Title         string   `bson:"title" json:"title"`
	Category      string   `bson:"category" json:"category"` // "communication" | "study" | "what_if" | "contradiction" | "seniority" | "jd_match" | "presence"
	Severity      string   `bson:"severity" json:"severity"` // "success" | "warning" | "info"
	Description   string   `bson:"description" json:"description"`
	TargetLevel   string   `bson:"target_level,omitempty" json:"target_level,omitempty"`
	ObservedLevel string   `bson:"observed_level,omitempty" json:"observed_level,omitempty"`
	ActionPoints  []string `bson:"action_points,omitempty" json:"action_points,omitempty"`
}

// CandidateCoaching represents the candidate growth guide.
type CandidateCoaching struct {
	ID          bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	InterviewID bson.ObjectID  `bson:"interview_id" json:"interview_id"`
	Items       []CoachingItem `bson:"items" json:"items"`
	CreatedAt   time.Time      `bson:"created_at" json:"created_at"`
}

// PondQuestion is a reusable LLM-generated question stored in the question pond.
type PondQuestion struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Question           string        `bson:"question" json:"question"`
	TargetCompetencies []string      `bson:"target_competencies" json:"target_competencies"`

	// Filter/category fields.
	Role         string `bson:"role" json:"role"`                   // interview Level
	Type         string `bson:"type" json:"type"`                   // interview Type
	RigorPercent int    `bson:"rigor_percent" json:"rigor_percent"` // difficulty when generated

	// Provenance / future-proofing meta (captured at insert; cheap to store, useful later
	// for filtering, analytics, dedup, and tracing where a question came from).
	Model             string        `bson:"model" json:"model"`                             // generating model, e.g. gemma4:e4b / claude-sonnet-4-6
	SourceInterviewID bson.ObjectID `bson:"source_interview_id" json:"source_interview_id"` // interview it was generated for (provenance only, NOT a reuse link)
	JobTitle          string        `bson:"job_title,omitempty" json:"job_title,omitempty"` // JD title at generation time (context)
	UsedCount         int           `bson:"used_count" json:"used_count"`                   // times reused from the pond; drives least-used rotation (incremented on reuse)
	CreatedAt         time.Time     `bson:"created_at" json:"created_at"`                   // when added to the pond
}
