package interview

import (
	"context"
	"strings"
	"testing"
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

type mockCaller struct {
	model string
	calls []string
}

func (m *mockCaller) Call(ctx context.Context, system, user string) (string, error) {
	m.calls = append(m.calls, user)
	if strings.Contains(system, "questions") || strings.Contains(system, "interview guide") || strings.Contains(user, "questions") {
		return `{"questions":[{"question":"Explain channels in Go.","target_competencies":["Go Concurrency"]},{"question":"Explain select block.","target_competencies":["Go Concurrency"]},{"question":"Explain context.","target_competencies":["Go Concurrency"]}]}`, nil
	}
	return `{"validation_targets":[{"competency":"Go Concurrency","priority":0.8}],"target":[{"name":"Go Concurrency","weight":0.9}]}`, nil
}

func (m *mockCaller) ModelName() string {
	return m.model
}

func mustInsert(t *testing.T, ctx context.Context, st *store.Store, coll string, doc any) {
	t.Helper()
	if _, err := st.Coll(coll).InsertOne(ctx, doc); err != nil {
		t.Fatalf("insert %s failed: %v", coll, err)
	}
}

func TestQuestionPondFlow(t *testing.T) {
	ctx := context.Background()
	dbName := "rejected_ai_test_pond_" + bson.NewObjectID().Hex()
	st, err := store.Connect(ctx, "mongodb://localhost:27017", dbName)
	if err != nil {
		t.Skipf("mongo not available, skipping: %v", err)
	}
	t.Cleanup(func() {
		_ = st.DB.Drop(context.Background())
		_ = st.Disconnect(context.Background())
	})

	// Seed JD and Candidate Profile
	jdID := bson.NewObjectID()
	cpID := bson.NewObjectID()
	mustInsert(t, ctx, st, store.CollJobDescriptions, domain.JobDescription{
		ID:    jdID,
		Title: "Backend Engineer",
		Raw:   "We need a backend developer with Go experience.",
	})
	mustInsert(t, ctx, st, store.CollCandidateProfile, domain.CandidateProfile{
		ID:   cpID,
		Name: "Ada Lovelace",
		Raw:  "Backend engineer skilled in Go and concurrency.",
	})

	// Mock LLM Responses: 1) Graph build, 2) Question generation
	mock := &mockCaller{
		model: "gemma4:e4b",
	}
	provider := &llm.Provider{Caller: mock}

	// Setup Services
	capSvc := capability.NewService(provider)
	evSvc := evidence.NewService(provider, st)
	confSvc := confidence.NewService(provider, st, evSvc)
	asmSvc := assumptions.NewService(provider)
	svc := NewService(provider, st, capSvc, evSvc, confSvc, asmSvc)

	// --- 1. AI MODE CREATION (should seed pond) ---
	reqAI := CreateRequest{
		JobDescriptionID:   jdID.Hex(),
		CandidateProfileID: cpID.Hex(),
		Level:              "Senior Engineer",
		Type:               "Coding",
		DurationMin:        10, // ~3 questions
		Source:             "ai",
	}

	resAI, err := svc.CreateSession(ctx, reqAI)
	if err != nil {
		t.Fatalf("failed to create AI session: %v", err)
	}
	if resAI.QuestionSource != "ai" {
		t.Errorf("expected source 'ai', got %q", resAI.QuestionSource)
	}

	// Verify questions were cached in CollQuestionsPond
	count, err := st.Coll(store.CollQuestionsPond).CountDocuments(ctx, bson.D{})
	if err != nil {
		t.Fatalf("count pond failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 questions in pond, got %d", count)
	}

	// Check fields on a pond question
	var pq domain.PondQuestion
	if err := st.Coll(store.CollQuestionsPond).FindOne(ctx, bson.D{}).Decode(&pq); err != nil {
		t.Fatalf("load pond question failed: %v", err)
	}
	if pq.Role != "Senior Engineer" || pq.Type != "Coding" {
		t.Errorf("incorrect role/type tags: %s/%s", pq.Role, pq.Type)
	}
	if pq.Model != "gemma4:e4b" || pq.JobTitle != "Backend Engineer" {
		t.Errorf("incorrect metadata: model=%s job=%s", pq.Model, pq.JobTitle)
	}

	// --- 2. POND MODE CREATION (should reuse, skip LLM, and build graph in bg) ---
	reqPond := CreateRequest{
		JobDescriptionID:   jdID.Hex(),
		CandidateProfileID: cpID.Hex(),
		Level:              "Senior Engineer",
		Type:               "Coding",
		DurationMin:        10,
		Source:             "pond",
	}

	resPond, err := svc.CreateSession(ctx, reqPond)
	if err != nil {
		t.Fatalf("failed to create Pond session: %v", err)
	}
	if resPond.QuestionSource != "pond" {
		t.Errorf("expected source 'pond', got %q", resPond.QuestionSource)
	}
	if resPond.Graphs != nil {
		t.Errorf("expected nil synchronous graphs in Pond mode, got %+v", resPond.Graphs)
	}
	if resPond.Interview.GraphStatus != domain.GraphStatusBuilding {
		t.Errorf("expected GraphStatus 'building', got %q", resPond.Interview.GraphStatus)
	}

	// Wait briefly for background capability build goroutine to finish
	time.Sleep(200 * time.Millisecond)

	// Check if graph was built in background
	var ivCheck domain.Interview
	if err := st.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: resPond.Interview.ID}}).Decode(&ivCheck); err != nil {
		t.Fatalf("load interview failed: %v", err)
	}
	if ivCheck.GraphStatus != domain.GraphStatusReady {
		t.Errorf("expected background build GraphStatus 'ready', got %q", ivCheck.GraphStatus)
	}
	if len(ivCheck.Competencies) == 0 {
		t.Error("expected populated interview competencies after background build")
	}

	// Verify used_count was incremented on the selected questions
	var pqCheck domain.PondQuestion
	if err := st.Coll(store.CollQuestionsPond).FindOne(ctx, bson.D{{Key: "question", Value: resPond.Question.Question}}).Decode(&pqCheck); err != nil {
		t.Fatalf("load selected pond question failed: %v", err)
	}
	if pqCheck.UsedCount != 1 {
		t.Errorf("expected UsedCount 1, got %d", pqCheck.UsedCount)
	}

	// --- 3. FALLBACK TO AI MODE (when filtering mismatch) ---
	reqFallback := CreateRequest{
		JobDescriptionID:   jdID.Hex(),
		CandidateProfileID: cpID.Hex(),
		Level:              "Staff Engineer", // no questions in pond for Staff
		Type:               "Coding",
		DurationMin:        10,
		Source:             "pond",
	}

	resFallback, err := svc.CreateSession(ctx, reqFallback)
	if err != nil {
		t.Fatalf("failed to create fallback session: %v", err)
	}
	if resFallback.QuestionSource != "ai_fallback" {
		t.Errorf("expected source 'ai_fallback', got %q", resFallback.QuestionSource)
	}
}

func TestSelectLeastUsed(t *testing.T) {
	questions := []domain.PondQuestion{
		{Question: "Q1", UsedCount: 2},
		{Question: "Q2", UsedCount: 0},
		{Question: "Q3", UsedCount: 1},
		{Question: "Q4", UsedCount: 0},
		{Question: "Q5", UsedCount: 1},
	}

	// select 2: should definitely get Q2 and Q4 (used_count = 0)
	sel := selectLeastUsed(questions, 2)
	if len(sel) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(sel))
	}

	hasQ2 := false
	hasQ4 := false
	for _, q := range sel {
		if q.Question == "Q2" {
			hasQ2 = true
		}
		if q.Question == "Q4" {
			hasQ4 = true
		}
	}
	if !hasQ2 || !hasQ4 {
		t.Errorf("expected Q2 and Q4, selected: %s, %s", sel[0].Question, sel[1].Question)
	}

	// select 4: should get Q2, Q4, plus two of (Q3, Q5)
	sel4 := selectLeastUsed(questions, 4)
	if len(sel4) != 4 {
		t.Fatalf("expected 4 questions, got %d", len(sel4))
	}
	hasQ1 := false
	for _, q := range sel4 {
		if q.Question == "Q1" {
			hasQ1 = true
		}
	}
	if hasQ1 {
		t.Error("Q1 (highest used_count) should not have been selected")
	}
}
