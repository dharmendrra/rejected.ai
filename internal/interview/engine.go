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

// AnswerResult is returned after processing an answer.
type AnswerResult struct {
	Turn      *domain.Turn                `json:"turn"`
	Evidence  []domain.EvidenceItem       `json:"evidence"`
	Snapshots []domain.ConfidenceSnapshot `json:"snapshots"`
	Next      *domain.Turn                `json:"next,omitempty"`
	Completed bool                        `json:"completed"`
}

// SubmitAnswer records an answer to the current open question,
// then either asks the next pre-generated question or completes the interview.
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

	if _, err := s.Store.Coll(store.CollQuestions).UpdateByID(ctx, current.ID, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "answer", Value: current.Answer},
			{Key: "answered", Value: true},
			{Key: "answered_at", Value: current.AnsweredAt},
		}},
	}); err != nil {
		return nil, fmt.Errorf("update answered turn: %w", err)
	}

	result := &AnswerResult{Turn: current}

	// Check if there are any remaining unanswered questions pre-generated in the database.
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].Turn < ts[j].Turn
	})

	var nextQuestion *domain.Turn
	for i := range ts {
		if ts[i].Turn == current.Turn+1 {
			nextQuestion = &ts[i]
			break
		}
	}

	if nextQuestion == nil {
		// No more pre-generated questions. Complete the interview.
		if err := s.complete(ctx, interviewID); err != nil {
			return nil, err
		}
		result.Completed = true
		return result, nil
	}

	result.Next = nextQuestion
	return result, nil
}

// generateAllQuestions generates all questions upfront at the start of the interview.
func (s *Service) generateAllQuestions(ctx context.Context, iv *domain.Interview, graphs *domain.CapabilityGraphSet) ([]domain.Turn, error) {
	n := questionBudget(iv.DurationMin)

	systemPrompt := fmt.Sprintf(`You are a seasoned technical interviewer generating a set of interview questions.
Rules:
- Never ask generic textbook questions. Probe for validation of the specific gaps and unknowns identified for THIS candidate and role.
- Prioritize competencies with low confidence and high validation priority.
- Generate exactly %d sequential, distinct questions. They should cover different target competencies without overlap, forming a structured, comprehensive interview guide.
- Ask single, focused questions.
Respond with a single JSON object containing an array of questions, and no prose.`, n)

	userPrompt := fmt.Sprintf(`Interview level: %s | type: %s
Generate exactly %d questions.

Validation targets (highest priority first):
%s

Generate the list of questions as JSON:
{
  "questions": [
    {
      "question": string,
      "target_competencies": string[]
    }
  ]
}`, iv.Level, iv.Type, n, llm.MarshalCompact(graphs.ValidationTargets))

	var res struct {
		Questions []struct {
			Question           string   `json:"question"`
			TargetCompetencies []string `json:"target_competencies"`
		} `json:"questions"`
	}

	if err := llm.CallJSON(ctx, s.LLM.Caller, systemPrompt, userPrompt, &res); err != nil {
		return nil, fmt.Errorf("generate all questions: %w", err)
	}

	turns := make([]domain.Turn, 0, len(res.Questions))
	for i, q := range res.Questions {
		turn := domain.Turn{
			InterviewID:        iv.ID,
			Turn:               i + 1,
			Kind:               domain.TurnQuestion,
			Question:           strings.TrimSpace(q.Question),
			TargetCompetencies: q.TargetCompetencies,
			AskedAt:            time.Now().UTC(),
		}
		turns = append(turns, turn)
	}
	return turns, nil
}

// EvaluateAllTurns runs evidence extraction, response analysis, and confidence rescoring
// for all answered questions in the interview. This is called when generating the report
// since evaluation is deferred.
func (s *Service) EvaluateAllTurns(ctx context.Context, interviewID bson.ObjectID, onSubStepStart ...func(int, string)) error {
	var stepCB func(int, string)
	if len(onSubStepStart) > 0 && onSubStepStart[0] != nil {
		stepCB = onSubStepStart[0]
	}

	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: interviewID}}).Decode(&iv); err != nil {
		return fmt.Errorf("load interview: %w", err)
	}

	ts, err := s.turns(ctx, interviewID)
	if err != nil {
		return err
	}

	// Make sure we evaluate each turn in order.
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].Turn < ts[j].Turn
	})

	for i := 0; i < len(ts); i++ {
		turn := &ts[i]
		if !turn.Answered {
			continue
		}

		// Check if this turn is already evaluated.
		// A turn is considered fully evaluated if we have existing confidence snapshots in the DB
		// and it has a response type set.
		var countConf int64
		countConf, _ = s.Store.Coll(store.CollConfidenceScores).CountDocuments(ctx, bson.D{
			{Key: "interview_id", Value: interviewID},
			{Key: "turn", Value: turn.Turn},
		})
		if countConf > 0 && turn.ResponseType != "" {
			continue
		}

		// Since we are evaluating this turn now, clear any partial/previous evidence or scores for this specific turn.
		_, _ = s.Store.Coll(store.CollConfidenceScores).DeleteMany(ctx, bson.D{
			{Key: "interview_id", Value: interviewID},
			{Key: "turn", Value: turn.Turn},
		})
		_, _ = s.Store.Coll(store.CollEvidenceLedger).DeleteMany(ctx, bson.D{
			{Key: "interview_id", Value: interviewID},
			{Key: "turn", Value: turn.Turn},
		})

		// Rebuild memory up to this turn.
		memory := buildMemory(ts[:i])

		if stepCB != nil {
			stepCB(turn.Turn, "Evidence Extraction")
		}
		items, err := s.Evidence.Extract(ctx, &iv, turn, memory)
		if err != nil {
			return fmt.Errorf("extract evidence turn %d: %w", turn.Turn, err)
		}

		if stepCB != nil {
			stepCB(turn.Turn, "Response Analysis")
		}
		analysis, err := s.Assumptions.Analyze(ctx, turn, memory)
		if err != nil {
			return fmt.Errorf("analyze turn %d: %w", turn.Turn, err)
		}
		turn.Assumptions = analysis.Assumptions
		turn.ResponseType = analysis.ResponseType
		turn.ResponseReasoning = analysis.Reasoning

		turn.CompressionRatio = signals.CompressionRatio(turn.Answer, len(items))

		if _, err := s.Store.Coll(store.CollQuestions).UpdateByID(ctx, turn.ID, bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "compression_ratio", Value: turn.CompressionRatio},
				{Key: "assumptions", Value: turn.Assumptions},
				{Key: "response_type", Value: turn.ResponseType},
				{Key: "response_reasoning", Value: turn.ResponseReasoning},
			}},
		}); err != nil {
			return fmt.Errorf("update turn %d: %w", turn.Turn, err)
		}

		if stepCB != nil {
			stepCB(turn.Turn, "Confidence Rescoring")
		}
		_, err = s.Confidence.Rescore(ctx, &iv, turn)
		if err != nil {
			return fmt.Errorf("rescore turn %d: %w", turn.Turn, err)
		}
	}
	return nil
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

// openTurn returns the first unanswered question, if any.
func openTurn(ts []domain.Turn) *domain.Turn {
	sort.Slice(ts, func(i, j int) bool {
		return ts[i].Turn < ts[j].Turn
	})
	for i := 0; i < len(ts); i++ {
		if !ts[i].Answered {
			t := ts[i]
			return &t
		}
	}
	return nil
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
