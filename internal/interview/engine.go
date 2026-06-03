package interview

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/signals"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const questionSystem = `You are a seasoned, adaptive technical interviewer. You generate ONE next question at a
time. Rules:
- Never ask generic textbook questions. Probe for validation of the specific gaps and
  unknowns identified for THIS candidate and role.
- Prioritize competencies with low confidence and high validation priority.
- Use the conversation so far; build on prior answers, don't repeat ground already covered.
- Match depth to the interview level and type, and to the time remaining.
- Ask a single, focused question.
Respond with a single JSON object and no prose.`

const questionUserTmpl = `Interview level: %s | type: %s
This is question %d of approximately %d.

Validation targets (highest priority first):
%s

Current confidence by competency (normal lens, 0..1; lower = needs validation):
%s

Conversation so far:
%s

Generate the next question as JSON:
{ "question": string, "target_competencies": string[], "rationale": string }`

type questionResult struct {
	Question           string   `json:"question"`
	TargetCompetencies []string `json:"target_competencies"`
	Rationale          string   `json:"rationale"`
}

// generateQuestion creates, persists, and returns the next question turn.
func (s *Service) generateQuestion(ctx context.Context, iv *domain.Interview, graphs *domain.CapabilityGraphSet, latest map[string]domain.ConfidenceSnapshot, turnNum int) (*domain.Turn, error) {
	ts, err := s.turns(ctx, iv.ID)
	if err != nil {
		return nil, err
	}

	user := fmt.Sprintf(questionUserTmpl,
		iv.Level, iv.Type, turnNum, questionBudget(iv.DurationMin),
		llm.MarshalCompact(graphs.ValidationTargets),
		renderConfidenceGaps(latest),
		buildMemory(ts),
	)

	var qr questionResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, questionSystem, user, &qr); err != nil {
		return nil, fmt.Errorf("generate question: %w", err)
	}

	turn := domain.Turn{
		InterviewID:        iv.ID,
		Turn:               turnNum,
		Kind:               domain.TurnQuestion,
		Question:           strings.TrimSpace(qr.Question),
		TargetCompetencies: qr.TargetCompetencies,
		AskedAt:            time.Now().UTC(),
	}
	res, err := s.Store.Coll(store.CollQuestions).InsertOne(ctx, turn)
	if err != nil {
		return nil, fmt.Errorf("persist question: %w", err)
	}
	turn.ID = res.InsertedID.(bson.ObjectID)
	return &turn, nil
}

// AnswerResult is returned after processing an answer.
type AnswerResult struct {
	Turn      *domain.Turn                `json:"turn"`
	Evidence  []domain.EvidenceItem       `json:"evidence"`
	Snapshots []domain.ConfidenceSnapshot `json:"snapshots"`
	Next      *domain.Turn                `json:"next,omitempty"`
	Completed bool                        `json:"completed"`
}

// SubmitAnswer records an answer to the current open question, runs evidence
// extraction and confidence re-scoring, then either asks the next question or
// completes the interview when the time budget is exhausted.
func (s *Service) SubmitAnswer(ctx context.Context, interviewID bson.ObjectID, answer string) (*AnswerResult, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil, fmt.Errorf("answer is empty")
	}

	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
		return nil, fmt.Errorf("load interview: %w", err)
	}
	if iv.Status == domain.StatusCompleted {
		return nil, fmt.Errorf("interview already completed")
	}

	ts, err := s.turns(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	current := openTurn(ts)
	if current == nil {
		return nil, fmt.Errorf("no open question to answer")
	}

	// Record the answer.
	current.Answer = answer
	current.Answered = true
	current.AnsweredAt = time.Now().UTC()

	memory := buildMemory(ts)

	items, err := s.Evidence.Extract(ctx, &iv, current, memory)
	if err != nil {
		return nil, err
	}

	// Per-answer assumption + clarification/deflection analysis (Phase 5).
	analysis, err := s.Assumptions.Analyze(ctx, current, memory)
	if err != nil {
		return nil, err
	}
	current.Assumptions = analysis.Assumptions
	current.ResponseType = analysis.ResponseType
	current.ResponseReasoning = analysis.Reasoning

	current.CompressionRatio = signals.CompressionRatio(answer, len(items))
	if _, err := s.Store.Coll(store.CollQuestions).UpdateByID(ctx, current.ID, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "answer", Value: current.Answer},
			{Key: "answered", Value: true},
			{Key: "answered_at", Value: current.AnsweredAt},
			{Key: "compression_ratio", Value: current.CompressionRatio},
			{Key: "assumptions", Value: current.Assumptions},
			{Key: "response_type", Value: current.ResponseType},
			{Key: "response_reasoning", Value: current.ResponseReasoning},
		}},
	}); err != nil {
		return nil, fmt.Errorf("update answered turn: %w", err)
	}

	snaps, err := s.Confidence.Rescore(ctx, &iv, current)
	if err != nil {
		return nil, err
	}

	result := &AnswerResult{Turn: current, Evidence: items, Snapshots: snaps}

	// Decide whether to continue.
	answered := countAnswered(ts) + 1 // include the one just answered
	if answered >= questionBudget(iv.DurationMin) {
		if err := s.complete(ctx, interviewID); err != nil {
			return nil, err
		}
		result.Completed = true
		return result, nil
	}

	latest, err := s.latestConfidence(ctx, interviewID)
	if err != nil {
		return nil, err
	}

	// Seek clarification before scoring down: a low-confidence targeted
	// competency (or a deflection) triggers a focused follow-up.
	if comp, ok := chooseFollowupCompetency(current, latest, ts); ok {
		next, err := s.generateFollowup(ctx, &iv, comp, current.Turn+1)
		if err != nil {
			return nil, err
		}
		result.Next = next
		return result, nil
	}

	graphs, err := s.loadGraphs(ctx, interviewID)
	if err != nil {
		return nil, err
	}
	next, err := s.generateQuestion(ctx, &iv, graphs, latest, current.Turn+1)
	if err != nil {
		return nil, err
	}
	result.Next = next
	return result, nil
}

func (s *Service) complete(ctx context.Context, interviewID bson.ObjectID) error {
	_, err := s.Store.Coll(store.CollInterviews).UpdateByID(ctx, interviewID, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: domain.StatusCompleted},
			{Key: "updated_at", Value: time.Now().UTC()},
		}},
	})
	return err
}

func (s *Service) loadGraphs(ctx context.Context, interviewID bson.ObjectID) (*domain.CapabilityGraphSet, error) {
	var g domain.CapabilityGraphSet
	if err := s.Store.Coll(store.CollCapabilityGraphs).FindOne(ctx, bson.D{{Key: "interview_id", Value: interviewID}}).Decode(&g); err != nil {
		return nil, fmt.Errorf("load graphs: %w", err)
	}
	return &g, nil
}

// openTurn returns the latest unanswered question, if any.
func openTurn(ts []domain.Turn) *domain.Turn {
	for i := len(ts) - 1; i >= 0; i-- {
		if !ts[i].Answered {
			t := ts[i]
			return &t
		}
	}
	return nil
}

func countAnswered(ts []domain.Turn) int {
	n := 0
	for _, t := range ts {
		if t.Answered {
			n++
		}
	}
	return n
}

func renderConfidenceGaps(latest map[string]domain.ConfidenceSnapshot) string {
	if len(latest) == 0 {
		return "(no evidence yet)"
	}
	type kv struct {
		comp  string
		score float64
	}
	rows := make([]kv, 0, len(latest))
	for c, s := range latest {
		rows = append(rows, kv{c, s.Normal})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].score < rows[j].score })
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "- %s: %.2f\n", r.comp, r.score)
	}
	return strings.TrimSpace(b.String())
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
