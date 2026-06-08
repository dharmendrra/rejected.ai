package report

import (
	"context"
	"testing"
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
)

// seqCaller returns successive scripted responses, one per call, in order.
type seqCaller struct {
	resps []string
	i     int
}

func (s *seqCaller) Call(ctx context.Context, system, user string) (string, error) {
	r := s.resps[s.i]
	s.i++
	return r, nil
}
func (s *seqCaller) ModelName() string { return "seq" }

// TestReportPipeline proves the Phase 6–7 orchestration: finalize scores → persona
// panel → signals → risk → recommendation, assembled and persisted.
func TestReportPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbName := "rejected_ai_test_" + bson.NewObjectID().Hex()
	st, err := store.Connect(ctx, "mongodb://localhost:27017", dbName)
	if err != nil {
		t.Skipf("mongo not available: %v", err)
	}
	t.Cleanup(func() {
		_ = st.DB.Drop(context.Background())
		_ = st.Disconnect(context.Background())
	})

	ivID := bson.NewObjectID()
	now := time.Now().UTC()

	// Seed interview, graphs, confidence snapshots, and evidence.
	mustInsert(t, ctx, st, store.CollInterviews, domain.Interview{
		ID: ivID, Level: "Senior", Type: "Mixed", Status: domain.StatusCompleted,
		Competencies: []string{"idempotency", "leadership"}, CreatedAt: now,
	})
	mustInsert(t, ctx, st, store.CollCapabilityGraphs, domain.CapabilityGraphSet{
		InterviewID: ivID,
		Target:      []domain.TargetCapability{{Name: "idempotency", Importance: "required", Weight: 0.9}},
		Gaps:        []string{"security"},
	})
	// idempotency: turn1=0.40 then turn4=0.85 (latest wins); leadership: turn2=0.60.
	for _, sn := range []domain.ConfidenceSnapshot{
		{InterviewID: ivID, Competency: "idempotency", Turn: 1, Normal: 0.40, Cool: 0.5, Hot: 0.3, Confidence: 0.40, EvidenceTurns: []int{1}},
		{InterviewID: ivID, Competency: "idempotency", Turn: 4, Normal: 0.85, Cool: 0.9, Hot: 0.7, Confidence: 0.85, EvidenceTurns: []int{1, 4}},
		{InterviewID: ivID, Competency: "leadership", Turn: 2, Normal: 0.60, Cool: 0.65, Hot: 0.5, Confidence: 0.60, EvidenceTurns: []int{2}},
	} {
		mustInsert(t, ctx, st, store.CollConfidenceScores, sn)
	}
	mustInsert(t, ctx, st, store.CollEvidenceLedger, domain.EvidenceItem{
		InterviewID: ivID, Turn: 4, Competency: "idempotency", Polarity: domain.PolarityPositive,
		Strength: 0.8, SupportingQuote: "exactly-once dedup key",
	})

	mock := &seqCaller{resps: []string{
		// 1) persona panel
		`{"personas":[{"persona":"Architect","overall_take":"solid reliability reasoning","endorsements":["idempotency"],"concerns":["security unproven"],"per_competency":[{"competency":"idempotency","score":0.85,"reasoning":"exactly-once shown"}]}]}`,
		// 2) signals
		`{"signals":[{"name":"reliability thinking","description":"exactly-once design","evidence_turns":[4]}]}`,
		// 3) risk
		`{"risks":[{"competency":"security","category":"missing","severity":"high","reason":"never discussed","evidence_turns":[]},{"competency":"leadership","category":"weak","severity":"medium","reason":"thin evidence","evidence_turns":[2]}]}`,
		// 4) recommendation
		`{"decision":"hire_with_risks","confidence_level":0.72,"reasoning":"strong reliability, security unproven","citations":[{"competency":"idempotency","turns":[1,4],"note":"retroactively validated"}]}`,
		// 5) ideal responses (skipped — no answered turns in the test seeding)
		// 5) coaching items
		`{"coaching_items":[{"title":"Study Idempotency Patterns","category":"study","severity":"warning","description":"Deepen understanding of exactly-once semantics.","action_points":["Read about idempotent consumer pattern"]}]}`,
	}}
	provider := &llm.Provider{Caller: mock}

	ev := evidence.NewService(provider, st)
	conf := confidence.NewService(provider, st, ev)
	svc := NewService(st, ev, conf,
		evaluators.NewService(provider), signals.NewService(provider),
		risk.NewService(provider), recommendation.NewService(provider), provider, capability.NewService(provider))

	rep, err := svc.Generate(ctx, ivID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Final competency scores use the latest snapshot per competency.
	got := map[string]float64{}
	for _, sc := range rep.CompetencyScores {
		got[sc.Competency] = sc.Normal
	}
	if got["idempotency"] != 0.85 {
		t.Errorf("idempotency final normal = %.2f, want 0.85 (latest snapshot)", got["idempotency"])
	}
	if got["leadership"] != 0.60 {
		t.Errorf("leadership final normal = %.2f, want 0.60", got["leadership"])
	}

	if len(rep.Signals) != 1 {
		t.Errorf("signals = %d, want 1", len(rep.Signals))
	}
	if len(rep.Risks) != 2 {
		t.Errorf("risks = %d, want 2", len(rep.Risks))
	}
	if rep.Recommendation == nil || rep.Recommendation.Decision != domain.DecisionHireWithRisks {
		t.Fatalf("recommendation = %+v, want hire_with_risks", rep.Recommendation)
	}
	if len(rep.Recommendation.Personas) != 1 {
		t.Errorf("personas embedded = %d, want 1", len(rep.Recommendation.Personas))
	}
	if mock.i != 5 {
		t.Errorf("expected exactly 5 LLM calls, got %d", mock.i)
	}

	// Everything persisted and reloadable.
	loaded, err := svc.Load(ctx, ivID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Recommendation == nil || loaded.Recommendation.Decision != domain.DecisionHireWithRisks {
		t.Errorf("reloaded recommendation missing/wrong: %+v", loaded.Recommendation)
	}
	if len(loaded.CompetencyScores) != 2 || len(loaded.Signals) != 1 || len(loaded.Risks) != 2 {
		t.Errorf("reload counts: scores=%d signals=%d risks=%d", len(loaded.CompetencyScores), len(loaded.Signals), len(loaded.Risks))
	}
	t.Logf("report OK: decision=%s, idempotency final=%.2f, risks=%d, personas=%d",
		rep.Recommendation.Decision, got["idempotency"], len(rep.Risks), len(rep.Recommendation.Personas))
}

func mustInsert(t *testing.T, ctx context.Context, st *store.Store, coll string, doc any) {
	t.Helper()
	if _, err := st.Coll(coll).InsertOne(ctx, doc); err != nil {
		t.Fatalf("seed %s: %v", coll, err)
	}
}
