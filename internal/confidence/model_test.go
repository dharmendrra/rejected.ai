package confidence

import (
	"context"
	"testing"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// mockCaller returns a fixed response, ignoring the prompt. It lets us prove the
// retroactive re-scoring mechanism deterministically, without depending on the
// speed or output quality of a real local model.
type mockCaller struct{ resp string }

func (m *mockCaller) Call(ctx context.Context, system, user string) (string, error) {
	return m.resp, nil
}
func (m *mockCaller) ModelName() string { return "mock" }

// TestRescore_RetroactiveRevision is the acceptance test for the defining
// behavior: an earlier evidence item's strength is revised UPWARD when a later
// answer clarifies it, and a confidence snapshot is stored for the turn.
func TestRescore_RetroactiveRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbName := "rejected_ai_test_" + bson.NewObjectID().Hex()
	st, err := store.Connect(ctx, "mongodb://localhost:27017", dbName)
	if err != nil {
		t.Skipf("mongo not available, skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = st.DB.Drop(context.Background())
		_ = st.Disconnect(context.Background())
	})

	interviewID := bson.NewObjectID()

	// Seed a single, deliberately weak/ambiguous evidence item from turn 1.
	seed := domain.EvidenceItem{
		InterviewID:     interviewID,
		Turn:            1,
		Competency:      "idempotency",
		Concepts:        []string{"duplicate handling"},
		Polarity:        domain.PolarityPositive,
		Strength:        0.30,
		SupportingQuote: "duplicate handling",
		Interpretation:  "ambiguous shorthand",
		CreatedAt:       time.Now().UTC(),
	}
	res, err := st.Coll(store.CollEvidenceLedger).InsertOne(ctx, seed)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	seedID := res.InsertedID.(bson.ObjectID)

	// The model "sees" the full ledger after turn 4 and revises e1 upward.
	mock := &mockCaller{resp: `{
      "competencies": [
        { "name": "idempotency", "cool": 0.9, "normal": 0.85, "hot": 0.7,
          "confidence": 0.85, "rationale": "later answer clarified the shorthand",
          "evidence_turns": [1, 4] }
      ],
      "revisions": [
        { "evidence_id": "e1", "new_strength": 0.8,
          "note": "turn 4 clarified 'duplicate handling' meant exactly-once idempotency" }
      ]
    }`}
	provider := &llm.Provider{Caller: mock}
	ev := evidence.NewService(provider, st)
	svc := NewService(provider, st, ev)

	iv := &domain.Interview{ID: interviewID, Level: "Senior", Type: "Mixed", Competencies: []string{"idempotency"}}
	turn4 := &domain.Turn{InterviewID: interviewID, Turn: 4, Question: "How did you ensure no double charges?", Answer: "dedup key, exactly-once; that's what I meant by duplicate handling"}

	snaps, err := svc.Rescore(ctx, iv, turn4)
	if err != nil {
		t.Fatalf("rescore: %v", err)
	}

	// Snapshot stored for the turn with the high confidence.
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Turn != 4 {
		t.Errorf("snapshot turn = %d, want 4", snaps[0].Turn)
	}
	if snaps[0].Normal < 0.8 {
		t.Errorf("normal confidence = %.2f, want >= 0.80", snaps[0].Normal)
	}

	// The retroactive proof: the turn-1 evidence item's strength was raised.
	var updated domain.EvidenceItem
	if err := st.Coll(store.CollEvidenceLedger).FindOne(ctx, bson.D{{Key: "_id", Value: seedID}}).Decode(&updated); err != nil {
		t.Fatalf("reload evidence: %v", err)
	}
	if updated.Strength <= 0.30 {
		t.Errorf("strength not revised upward: got %.2f, want > 0.30", updated.Strength)
	}
	if len(updated.Revisions) != 1 {
		t.Fatalf("want 1 revision, got %d", len(updated.Revisions))
	}
	rev := updated.Revisions[0]
	if rev.AtTurn != 4 || rev.OldStrength != 0.30 || rev.NewStrength != 0.80 {
		t.Errorf("revision = %+v, want at_turn=4 old=0.30 new=0.80", rev)
	}
	t.Logf("retroactive revision OK: turn-1 strength 0.30 -> %.2f (reason: %q)", updated.Strength, rev.Note)
}
