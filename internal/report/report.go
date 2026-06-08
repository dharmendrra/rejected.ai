// Package report orchestrates the final assessment: it finalizes competency
// scores, runs the evaluator panel, derives strongest signals and risk areas,
// and produces the explainable hiring recommendation — persisting each piece and
// assembling them into one read model.
package report

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dharmendra/rejected.ai/internal/capability"
	"github.com/dharmendra/rejected.ai/internal/confidence"
	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/evaluators"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/recommendation"
	"github.com/dharmendra/rejected.ai/internal/risk"
	"github.com/dharmendra/rejected.ai/internal/signals"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	LLM            *llm.Provider
	Capability     *capability.Service
}

// NewService wires the report Service.
func NewService(st *store.Store, ev *evidence.Service, conf *confidence.Service, eval *evaluators.Service, sig *signals.Service, rk *risk.Service, rec *recommendation.Service, provider *llm.Provider, cap *capability.Service) *Service {
	return &Service{Store: st, Evidence: ev, Confidence: conf, Evaluators: eval, Signals: sig, Risk: rk, Recommendation: rec, LLM: provider, Capability: cap}
}

// Report is the assembled final assessment read model.
type Report struct {
	Interview        *domain.Interview        `json:"interview"`
	CompetencyScores []domain.CompetencyScore `json:"competency_scores"`
	Signals          []domain.StrongestSignal `json:"signals"`
	Risks            []domain.RiskItem        `json:"risks"`
	Recommendation   *domain.Recommendation   `json:"recommendation"`
	IdealResponses   []domain.IdealResponse   `json:"ideal_responses"`
	CoachingItems    []domain.CoachingItem    `json:"coaching_items,omitempty"`
	Status           string                   `json:"status,omitempty"`
	Progress         *domain.ReportProgress   `json:"progress,omitempty"`
}

// Generate computes the full report, persists each component, and returns it.
func (s *Service) Generate(ctx context.Context, interviewID bson.ObjectID, onStepStart ...func(string)) (*Report, error) {
	var stepCB func(string)
	if len(onStepStart) > 0 && onStepStart[0] != nil {
		stepCB = onStepStart[0]
	}

	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
		return nil, fmt.Errorf("load interview: %w", err)
	}

	// Ensure capability graph is ready (Pond mode wait/sync-fallback).
	if iv.GraphStatus == "building" {
		timeout := time.After(20 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

	waitLoop:
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timeout:
				log.Printf("[REPORT] Timeout waiting for background graph build for interview %s, falling back to sync build", interviewID.Hex())
				break waitLoop
			case <-ticker.C:
				if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
					log.Printf("[REPORT] Failed to reload interview during graph wait: %v", err)
				}
				if iv.GraphStatus != "building" {
					break waitLoop
				}
			}
		}
	}

	var graphs domain.CapabilityGraphSet
	loadErr := s.Store.Coll(store.CollCapabilityGraphs).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&graphs)
	if loadErr != nil {
		// Sync fallback: build graph now.
		log.Printf("[REPORT] Graph not ready or missing for interview %s, building synchronously", interviewID.Hex())
		var jd domain.JobDescription
		if err := s.Store.Coll(store.CollJobDescriptions).FindOne(ctx, bson.D{{Key: "_id", Value: iv.JobDescriptionID}}).Decode(&jd); err != nil {
			return nil, fmt.Errorf("sync graph fallback: load job description: %w", err)
		}
		var cp domain.CandidateProfile
		if err := s.Store.Coll(store.CollCandidateProfile).FindOne(ctx, bson.D{{Key: "_id", Value: iv.CandidateProfileID}}).Decode(&cp); err != nil {
			return nil, fmt.Errorf("sync graph fallback: load candidate profile: %w", err)
		}

		syncGraphs, err := s.Capability.Build(ctx, iv.Level, iv.Type, &jd, &cp)
		if err != nil {
			return nil, fmt.Errorf("sync graph fallback build: %w", err)
		}
		syncGraphs.InterviewID = iv.ID
		syncGraphs.CreatedAt = time.Now().UTC()
		_, _ = s.Store.Coll(store.CollCapabilityGraphs).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: iv.ID}})
		_, err = s.Store.Coll(store.CollCapabilityGraphs).InsertOne(ctx, syncGraphs)
		if err != nil {
			return nil, fmt.Errorf("sync graph fallback save: %w", err)
		}
		graphs = *syncGraphs

		comps := capability.DeriveCompetencies(syncGraphs)
		_, _ = s.Store.Coll(store.CollInterviews).UpdateByID(ctx, iv.ID, bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "graph_status", Value: "ready"},
				{Key: "competencies", Value: comps},
				{Key: "updated_at", Value: time.Now().UTC()},
			}},
		})
	}

	scores, err := s.Confidence.Finalize(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	ledger, err := s.Evidence.All(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	// Load pre-existing documents for incremental resumption
	var recDoc domain.Recommendation
	errRec := s.Store.Coll(store.CollRecommendations).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&recDoc)

	var personas []domain.PersonaView
	if stepCB != nil {
		stepCB("Evaluator Personas")
	}
	if errRec == nil && len(recDoc.Personas) > 0 {
		personas = recDoc.Personas
	} else {
		personas, err = s.Evaluators.Evaluate(ctx, &iv, scores, ledger)
		if err != nil {
			return nil, err
		}
		// Save partial recommendation document containing only personas
		_, _ = s.Store.Coll(store.CollRecommendations).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
		_, err = s.Store.Coll(store.CollRecommendations).InsertOne(ctx, domain.Recommendation{
			InterviewID: interviewID,
			Personas:    personas,
			CreatedAt:   time.Now().UTC(),
		})
		if err != nil {
			return nil, fmt.Errorf("persist partial recommendation (personas): %w", err)
		}
	}

	var sigs []domain.StrongestSignal
	if stepCB != nil {
		stepCB("Strongest Signals")
	}
	var sigDoc domain.SignalsDoc
	errSig := s.Store.Coll(store.CollSignals).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&sigDoc)
	if errSig == nil && len(sigDoc.Signals) > 0 {
		sigs = sigDoc.Signals
	} else {
		sigs, err = s.Signals.Strongest(ctx, scores, ledger)
		if err != nil {
			return nil, err
		}
		_, _ = s.Store.Coll(store.CollSignals).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
		if _, err := s.Store.Coll(store.CollSignals).InsertOne(ctx, domain.SignalsDoc{InterviewID: interviewID, Signals: sigs, CreatedAt: time.Now().UTC()}); err != nil {
			return nil, fmt.Errorf("persist signals: %w", err)
		}
	}

	var risks []domain.RiskItem
	if stepCB != nil {
		stepCB("Risk Assessment")
	}
	var riskDoc domain.RiskDoc
	errRisk := s.Store.Coll(store.CollRiskAreas).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&riskDoc)
	if errRisk == nil && len(riskDoc.Risks) > 0 {
		risks = riskDoc.Risks
	} else {
		risks, err = s.Risk.Assess(ctx, &graphs, scores)
		if err != nil {
			return nil, err
		}
		_, _ = s.Store.Coll(store.CollRiskAreas).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
		if _, err := s.Store.Coll(store.CollRiskAreas).InsertOne(ctx, domain.RiskDoc{InterviewID: interviewID, Risks: risks, CreatedAt: time.Now().UTC()}); err != nil {
			return nil, fmt.Errorf("persist risks: %w", err)
		}
	}

	var rec *domain.Recommendation
	if stepCB != nil {
		stepCB("Hiring Recommendation")
	}
	if errRec == nil && recDoc.Decision != "" {
		rec = &recDoc
	} else {
		rec, err = s.Recommendation.Decide(ctx, &iv, scores, sigs, risks, personas)
		if err != nil {
			return nil, err
		}
		rec.Personas = personas
		rec.CreatedAt = time.Now().UTC()

		_, _ = s.Store.Coll(store.CollRecommendations).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
		if _, err := s.Store.Coll(store.CollRecommendations).InsertOne(ctx, rec); err != nil {
			return nil, fmt.Errorf("persist recommendation: %w", err)
		}
	}

	var turns []domain.Turn
	curTurns, err := s.Store.Coll(store.CollQuestions).Find(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
	if err == nil {
		_ = curTurns.All(ctx, &turns)
	}
	var answeredTurns []domain.Turn
	for _, t := range turns {
		if t.Answered {
			answeredTurns = append(answeredTurns, t)
		}
	}

	var idealRes []domain.IdealResponse
	if stepCB != nil {
		stepCB("Ideal Response Guide")
	}
	var idealDoc domain.IdealResponsesDoc
	errIdeal := s.Store.Coll(store.CollIdealResponses).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&idealDoc)
	if errIdeal == nil && len(idealDoc.Responses) > 0 {
		idealRes = idealDoc.Responses
	} else {
		idealRes, err = s.generateIdealResponses(ctx, &iv, answeredTurns)
		if err != nil {
			log.Printf("[REPORT] ideal responses generation failed (section omitted): %v", err)
		} else if len(idealRes) > 0 {
			_, _ = s.Store.Coll(store.CollIdealResponses).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
			if _, err := s.Store.Coll(store.CollIdealResponses).InsertOne(ctx, domain.IdealResponsesDoc{InterviewID: interviewID, Responses: idealRes, CreatedAt: time.Now().UTC()}); err != nil {
				return nil, fmt.Errorf("persist ideal responses: %w", err)
			}
		}
	}

	var coachingItems []domain.CoachingItem
	if stepCB != nil {
		stepCB("Candidate Coaching Guide")
	}
	var coachingDoc domain.CandidateCoaching
	errCoaching := s.Store.Coll(store.CollCandidateCoaching).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&coachingDoc)
	if errCoaching == nil && len(coachingDoc.Items) > 0 {
		coachingItems = coachingDoc.Items
	} else {
		coachingItems, err = s.generateCoachingItems(ctx, &iv, scores, ledger)
		if err != nil {
			log.Printf("[REPORT] coaching guide generation failed (section omitted): %v", err)
		} else if len(coachingItems) > 0 {
			_, _ = s.Store.Coll(store.CollCandidateCoaching).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: interviewID}})
			if _, err := s.Store.Coll(store.CollCandidateCoaching).InsertOne(ctx, domain.CandidateCoaching{
				ID:          bson.NewObjectID(),
				InterviewID: interviewID,
				Items:       coachingItems,
				CreatedAt:   time.Now().UTC(),
			}); err != nil {
				return nil, fmt.Errorf("persist coaching guide: %w", err)
			}
		}
	}

	_, _ = s.Store.Coll(store.CollInterviews).UpdateByID(ctx, interviewID, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: domain.StatusCompleted},
			{Key: "updated_at", Value: time.Now().UTC()},
		}},
	})

	return &Report{
		Interview:        &iv,
		CompetencyScores: scores,
		Signals:          sigs,
		Risks:            risks,
		Recommendation:   rec,
		IdealResponses:   idealRes,
		CoachingItems:    coachingItems,
	}, nil
}

// persist writes the signals, risk, and recommendation documents, replacing any
// prior copies so report regeneration is idempotent.
func (s *Service) persist(ctx context.Context, id bson.ObjectID, sigs []domain.StrongestSignal, risks []domain.RiskItem, rec *domain.Recommendation, idealRes []domain.IdealResponse) error {
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

	_, _ = s.Store.Coll(store.CollIdealResponses).DeleteMany(ctx, filter)
	if len(idealRes) > 0 {
		if _, err := s.Store.Coll(store.CollIdealResponses).InsertOne(ctx, domain.IdealResponsesDoc{InterviewID: id, Responses: idealRes, CreatedAt: now}); err != nil {
			return fmt.Errorf("persist ideal responses: %w", err)
		}
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

	var idealDoc domain.IdealResponsesDoc
	if err := s.Store.Coll(store.CollIdealResponses).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&idealDoc); err == nil {
		rep.IdealResponses = idealDoc.Responses
	}

	var coachingDoc domain.CandidateCoaching
	if err := s.Store.Coll(store.CollCandidateCoaching).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&coachingDoc); err == nil {
		rep.CoachingItems = coachingDoc.Items
	}

	if rep.Recommendation == nil {
		var progress domain.ReportProgress
		if err := s.Store.Coll(store.CollReportProgress).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&progress); err == nil {
			if progress.Status == "generating" || progress.Status == "failed" {
				rep.Status = progress.Status
				rep.Progress = &progress
				return rep, nil
			}
		}
	}

	return rep, nil
}

const idealSystemPrompt = `You are a Principal Engineering Interviewer. 
For each question asked in the interview, provide the ideal response guidelines that would score >85% confidence and competency ratings (Principal/Staff level depth).
For each question, return:
1. The target competency.
2. The key points/concepts the candidate MUST cover (e.g., specific technologies, architectural trade-offs, edge cases).
3. A realistic, high-scoring sample answer demonstrating this depth.

Respond with a single JSON object containing an array of "ideal_responses", and NO prose:
{
  "ideal_responses": [
    {
      "question": "The exact question asked",
      "competency": "The target competency name",
      "key_points": ["point 1", "point 2", ...],
      "sample_answer": "Detailed high-scoring sample answer"
    }
  ]
}`

const idealUserTmpl = `Interview Level: %s
Interview Type: %s

Questions and candidate answers:
%s

Generate the ideal response guide in JSON format.`

type idealResponsesResult struct {
	IdealResponses []domain.IdealResponse `json:"ideal_responses"`
}

// ─── Coaching Items Generation ────────────────────────────────────────────────

const coachingSystemPrompt = `You are an expert engineering interview coach.
Analyze the candidate's interview performance and produce actionable coaching items.

Each coaching item MUST have:
- "title": short label for the feedback item
- "category": one of "communication", "study", "what_if", "contradiction", "seniority", "jd_match", "presence"
- "severity": one of "success" (doing well), "warning" (needs work), "info" (nice-to-know)
- "description": 1-2 sentence explanation
- "action_points": array of concrete next-steps (can be empty for info/success items)

For the SENIORITY category specifically, also include:
- "target_level": the seniority level the role requires (e.g. "Staff Engineer", "Senior Engineer", "Principal Engineer")
- "observed_level": the seniority level the candidate demonstrated

Category guidelines:
- communication: How well the candidate articulated technical ideas, used precise terminology, and structured answers.
- study: Specific technical topics the candidate should study or deepen knowledge in.
- what_if: Hypothetical improvements — "If you had mentioned X, your score would have increased from Y% to Z%."
- contradiction: Logical inconsistencies between different answers.
- seniority: Gap between expected and demonstrated seniority level.
- jd_match: Skills or technologies from the job description that the candidate did or did not demonstrate.
- presence: Speaking pace, engagement, response latency, and visual presence metrics.

Return EXACTLY one item per category (7 items total).
Respond with a single JSON object and NO prose:
{
  "coaching_items": [
    {
      "title": "...",
      "category": "communication",
      "severity": "warning",
      "description": "...",
      "action_points": ["...", "..."]
    },
    {
      "title": "Seniority Gap",
      "category": "seniority",
      "severity": "warning",
      "description": "...",
      "target_level": "Staff Engineer",
      "observed_level": "Senior Engineer",
      "action_points": ["...", "..."]
    }
  ]
}`

const coachingUserTmpl = `Interview Level: %s
Interview Type: %s
Job Title: %s

Competency Scores:
%s

Evidence Summary:
%s

Generate the coaching items JSON.`

type coachingResult struct {
	CoachingItems []domain.CoachingItem `json:"coaching_items"`
}

func (s *Service) generateCoachingItems(ctx context.Context, iv *domain.Interview, scores []domain.CompetencyScore, ledger []domain.EvidenceItem) ([]domain.CoachingItem, error) {
	if len(scores) == 0 && len(ledger) == 0 {
		return nil, nil
	}

	var scoresStr strings.Builder
	for _, sc := range scores {
		scoresStr.WriteString(fmt.Sprintf("- %s: confidence=%.2f, cool=%.2f, normal=%.2f, hot=%.2f — %s\n",
			sc.Competency, sc.Confidence, sc.Cool, sc.Normal, sc.Hot, sc.Rationale))
	}

	var evidenceStr strings.Builder
	for _, ev := range ledger {
		evidenceStr.WriteString(fmt.Sprintf("- [Q%d] %s (%s, strength=%.2f): %s\n",
			ev.Turn, ev.Competency, ev.Polarity, ev.Strength, ev.Interpretation))
	}

	jobTitle := iv.Level + " " + iv.Type
	user := fmt.Sprintf(coachingUserTmpl, iv.Level, iv.Type, jobTitle, scoresStr.String(), evidenceStr.String())

	var res coachingResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, coachingSystemPrompt, user, &res); err != nil {
		return nil, err
	}
	return res.CoachingItems, nil
}

func (s *Service) generateIdealResponses(ctx context.Context, iv *domain.Interview, turns []domain.Turn) ([]domain.IdealResponse, error) {
	if len(turns) == 0 {
		return nil, nil
	}

	var turnsStr strings.Builder
	for _, t := range turns {
		turnsStr.WriteString(fmt.Sprintf("Question: %s\nTarget Competencies: %s\nCandidate Answer: %s\n---\n", 
			t.Question, strings.Join(t.TargetCompetencies, ", "), t.Answer))
	}

	user := fmt.Sprintf(idealUserTmpl, iv.Level, iv.Type, turnsStr.String())
	var res idealResponsesResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, idealSystemPrompt, user, &res); err != nil {
		return nil, err
	}
	return res.IdealResponses, nil
}

// UpdateProgress upserts a progress document for an interview.
func (s *Service) UpdateProgress(ctx context.Context, interviewID bson.ObjectID, currentStep string, steps []domain.ReportStep) {
	completed := 0
	for _, step := range steps {
		if step.Status == "completed" {
			completed++
		}
	}

	now := time.Now().UTC()
	filter := bson.D{{Key: "interview_id", Value: interviewID}}

	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: "generating"},
			{Key: "total_steps", Value: len(steps)},
			{Key: "completed_steps", Value: completed},
			{Key: "current_step", Value: currentStep},
			{Key: "steps", Value: steps},
			{Key: "updated_at", Value: now},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "created_at", Value: now},
		}},
	}

	opts := options.UpdateOne().SetUpsert(true)
	_, _ = s.Store.Coll(store.CollReportProgress).UpdateOne(ctx, filter, update, opts)
}

// CompleteProgress marks the report generation progress as completed.
func (s *Service) CompleteProgress(ctx context.Context, interviewID bson.ObjectID) {
	_, _ = s.Store.Coll(store.CollReportProgress).UpdateOne(ctx,
		bson.D{{Key: "interview_id", Value: interviewID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: "completed"},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
}

// FailProgress marks the report generation progress as failed with an error message.
func (s *Service) FailProgress(ctx context.Context, interviewID bson.ObjectID, errMsg string) {
	_, _ = s.Store.Coll(store.CollReportProgress).UpdateOne(ctx,
		bson.D{{Key: "interview_id", Value: interviewID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: "failed"},
			{Key: "error", Value: errMsg},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
}
