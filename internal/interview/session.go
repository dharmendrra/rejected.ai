// Package interview orchestrates the live interview: it creates a session from a
// JD + resume, builds the capability graphs, generates questions dynamically
// from validation targets and confidence gaps, and runs each answer through
// evidence extraction and confidence re-scoring.
package interview

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
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
	RigorPercent       int    `json:"rigor_percent"`
	Source             string `json:"source"` // "" / "ai" (default) or "pond"
}

// CreateResult is returned when an interview starts.
type CreateResult struct {
	Interview      *domain.Interview          `json:"interview"`
	Graphs         *domain.CapabilityGraphSet `json:"graphs"`
	Question       *domain.Turn               `json:"question"`
	QuestionSource string                     `json:"question_source,omitempty"` // "pond" | "ai" | "ai_fallback"
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
	if req.RigorPercent <= 0 {
		req.RigorPercent = 50
	}

	n := questionBudget(req.DurationMin)
	now := time.Now().UTC()

	// Branch based on source: "pond" or "ai" (default)
	if req.Source == "pond" {
		// Pull questions from the pond.
		cur, err := s.Store.Coll(store.CollQuestionsPond).Find(ctx, bson.D{
			{Key: "role", Value: req.Level},
			{Key: "type", Value: req.Type},
		})
		var pondQuestions []domain.PondQuestion
		if err == nil {
			_ = cur.All(ctx, &pondQuestions)
		}

		if len(pondQuestions) > 0 {
			// Select N questions using least-used rotation with random tiebreak.
			selectedPq := selectLeastUsed(pondQuestions, n)

			// Seed competencies from the union of selected questions' target_competencies.
			var seedCompetencies []string
			seenComp := make(map[string]bool)
			for _, pq := range selectedPq {
				for _, c := range pq.TargetCompetencies {
					key := normalize(c)
					if key != "" && !seenComp[key] {
						seenComp[key] = true
						seedCompetencies = append(seedCompetencies, c)
					}
				}
			}

			// Create the interview document
			iv := domain.Interview{
				JobDescriptionID:   jdID,
				CandidateProfileID: cpID,
				Level:              req.Level,
				Type:               req.Type,
				DurationMin:        req.DurationMin,
				RigorPercent:       req.RigorPercent,
				Status:             domain.StatusActive,
				GraphStatus:        domain.GraphStatusBuilding,
				Competencies:       seedCompetencies,
				CreatedAt:          now,
				UpdatedAt:          now,
			}

			res, err := s.Store.Coll(store.CollInterviews).InsertOne(ctx, iv)
			if err != nil {
				return nil, fmt.Errorf("persist interview: %w", err)
			}
			iv.ID = res.InsertedID.(bson.ObjectID)

			// Map pond questions to turns
			turns := make([]domain.Turn, 0, len(selectedPq))
			for i, pq := range selectedPq {
				turn := domain.Turn{
					InterviewID:        iv.ID,
					Turn:               i + 1,
					Kind:               domain.TurnQuestion,
					Question:           pq.Question,
					TargetCompetencies: pq.TargetCompetencies,
					AskedAt:            now,
				}
				turns = append(turns, turn)
			}

			docs := make([]any, len(turns))
			for i, t := range turns {
				docs[i] = t
			}
			res2, err := s.Store.Coll(store.CollQuestions).InsertMany(ctx, docs)
			if err != nil {
				return nil, fmt.Errorf("persist questions: %w", err)
			}
			for i := range turns {
				turns[i].ID = res2.InsertedIDs[i].(bson.ObjectID)
			}

			// Increment used_count for selected questions (best-effort)
			for _, pq := range selectedPq {
				_, _ = s.Store.Coll(store.CollQuestionsPond).UpdateByID(ctx, pq.ID, bson.D{
					{Key: "$inc", Value: bson.D{{Key: "used_count", Value: 1}}},
				})
			}

			// Launch background capability graph build
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				graphs, err := s.Capability.Build(bgCtx, req.Level, req.Type, &jd, &cp)
				if err != nil {
					log.Printf("[POND] Background capability graph build failed for interview %s: %v", iv.ID.Hex(), err)
					_, _ = s.Store.Coll(store.CollInterviews).UpdateByID(context.Background(), iv.ID, bson.D{
						{Key: "$set", Value: bson.D{
							{Key: "graph_status", Value: domain.GraphStatusFailed},
							{Key: "updated_at", Value: time.Now().UTC()},
						}},
					})
					return
				}

				graphs.InterviewID = iv.ID
				graphs.CreatedAt = time.Now().UTC()
				_, err = s.Store.Coll(store.CollCapabilityGraphs).InsertOne(bgCtx, graphs)
				if err != nil {
					log.Printf("[POND] Background capability graph persist failed for interview %s: %v", iv.ID.Hex(), err)
					_, _ = s.Store.Coll(store.CollInterviews).UpdateByID(context.Background(), iv.ID, bson.D{
						{Key: "$set", Value: bson.D{
							{Key: "graph_status", Value: domain.GraphStatusFailed},
							{Key: "updated_at", Value: time.Now().UTC()},
						}},
					})
					return
				}

				comps := capability.DeriveCompetencies(graphs)
				_, err = s.Store.Coll(store.CollInterviews).UpdateByID(context.Background(), iv.ID, bson.D{
					{Key: "$set", Value: bson.D{
						{Key: "graph_status", Value: domain.GraphStatusReady},
						{Key: "competencies", Value: comps},
						{Key: "updated_at", Value: time.Now().UTC()},
					}},
				})
				if err != nil {
					log.Printf("[POND] Background interview update failed for %s: %v", iv.ID.Hex(), err)
				}
			}()

			return &CreateResult{
				Interview:      &iv,
				Graphs:         nil, // build in background
				Question:       &turns[0],
				QuestionSource: "pond",
			}, nil
		}
		// If empty, fall through to AI fallback.
		log.Printf("[POND] No questions in pond for level %q type %q, falling back to AI generation", req.Level, req.Type)
	}

	// AI Mode (Default / Fallback)
	graphs, err := s.Capability.Build(ctx, req.Level, req.Type, &jd, &cp)
	if err != nil {
		return nil, err
	}

	competencies := capability.DeriveCompetencies(graphs)
	iv := domain.Interview{
		JobDescriptionID:   jdID,
		CandidateProfileID: cpID,
		Level:              req.Level,
		Type:               req.Type,
		DurationMin:        req.DurationMin,
		RigorPercent:       req.RigorPercent,
		Status:             domain.StatusActive,
		GraphStatus:        domain.GraphStatusReady,
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

	// Generate all questions upfront.
	turns, err := s.generateAllQuestions(ctx, &iv, graphs)
	if err != nil {
		return nil, err
	}

	docs := make([]any, len(turns))
	for i, t := range turns {
		docs[i] = t
	}
	res2, err := s.Store.Coll(store.CollQuestions).InsertMany(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("persist questions: %w", err)
	}
	for i := range turns {
		turns[i].ID = res2.InsertedIDs[i].(bson.ObjectID)
	}

	// Append generated questions to pond (best-effort)
	pondDocs := make([]any, len(turns))
	for i, t := range turns {
		pondDocs[i] = domain.PondQuestion{
			ID:                 bson.NewObjectID(),
			Question:           t.Question,
			TargetCompetencies: t.TargetCompetencies,
			Role:               iv.Level,
			Type:               iv.Type,
			RigorPercent:       iv.RigorPercent,
			Model:              s.LLM.Caller.ModelName(),
			SourceInterviewID:  iv.ID,
			JobTitle:           jd.Title,
			UsedCount:          0,
			CreatedAt:          now,
		}
	}
	if _, err := s.Store.Coll(store.CollQuestionsPond).InsertMany(ctx, pondDocs); err != nil {
		log.Printf("[POND] Failed to append generated questions to pond: %v", err)
	}

	source := "ai"
	if req.Source == "pond" {
		source = "ai_fallback"
	}

	return &CreateResult{
		Interview:      &iv,
		Graphs:         graphs,
		Question:       &turns[0],
		QuestionSource: source,
	}, nil
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

// selectLeastUsed selects n questions from a slice using least-used rotation with random tiebreaking.
func selectLeastUsed(questions []domain.PondQuestion, n int) []domain.PondQuestion {
	if len(questions) <= n {
		return questions
	}
	// Group by UsedCount
	groups := make(map[int][]domain.PondQuestion)
	for _, q := range questions {
		groups[q.UsedCount] = append(groups[q.UsedCount], q)
	}
	// Get sorted list of UsedCount keys
	var keys []int
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	// Shuffle each group and flatten
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var flattened []domain.PondQuestion
	for _, k := range keys {
		group := groups[k]
		r.Shuffle(len(group), func(i, j int) {
			group[i], group[j] = group[j], group[i]
		})
		flattened = append(flattened, group...)
	}

	return flattened[:n]
}
