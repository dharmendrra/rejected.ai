// Package confidence implements the defining behavior of the platform: a
// competency's confidence is a function of the ENTIRE evidence ledger,
// recomputed after every turn. Because each recompute sees all evidence —
// including later answers — earlier shorthand can be reinterpreted and its
// evidence strength revised upward. Retroactive re-scoring is therefore
// structural, not a special case.
package confidence

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
)

// Service recomputes competency confidence and persists per-turn snapshots.
type Service struct {
	LLM      *llm.Provider
	Store    *store.Store
	Evidence *evidence.Service
}

// NewService constructs a confidence Service.
func NewService(provider *llm.Provider, st *store.Store, ev *evidence.Service) *Service {
	return &Service{LLM: provider, Store: st, Evidence: ev}
}

const rescoreSystem = `You maintain evolving confidence scores for an interview. You are given the COMPLETE
evidence ledger (every item from every turn) and the most recent exchange. Re-assess all
competencies in light of EVERYTHING now known.

Three evaluation lenses, all required per competency (each 0..1):
- "cool": generous — allows reasonable inference from shorthand and intent.
- "normal": balanced — expects demonstrated, sufficient understanding.
- "hot": strict — rewards explicit terminology and rubric-level articulation (ATS-like).

"confidence" is your overall belief (0..1), aligned with the normal lens.

RETROACTIVE RE-SCORING: if a later answer clarifies or validates an earlier ambiguous
answer, revise that earlier evidence item's strength upward (or downward if a later answer
undermines it). Emit such changes in "revisions", referencing the evidence id. Only emit a
revision when the strength actually changes; include a short note explaining why.

Cite the turns that most support each competency in "evidence_turns".
Respond with a single JSON object and no prose.`

const rescoreUserTmpl = `Interview level: %s | type: %s
Most recent exchange (turn %d):
Q: %s
A: %s

COMPLETE EVIDENCE LEDGER (grouped by competency):
%s

Produce JSON:
{
  "competencies": [
    {
      "name": string,
      "cool": number, "normal": number, "hot": number, "confidence": number,
      "rationale": string,
      "evidence_turns": number[]
    }
  ],
  "revisions": [
    { "evidence_id": string, "new_strength": number(0..1), "note": string }
  ]
}`

type rescoreResult struct {
	Competencies []struct {
		Name          string  `json:"name"`
		Cool          float64 `json:"cool"`
		Normal        float64 `json:"normal"`
		Hot           float64 `json:"hot"`
		Confidence    float64 `json:"confidence"`
		Rationale     string  `json:"rationale"`
		EvidenceTurns []int   `json:"evidence_turns"`
	} `json:"competencies"`
	Revisions []struct {
		EvidenceID  string  `json:"evidence_id"`
		NewStrength float64 `json:"new_strength"`
		Note        string  `json:"note"`
	} `json:"revisions"`
}

// Rescore recomputes every competency from the full ledger after the given
// turn, applies any retroactive evidence revisions, and stores one confidence
// snapshot per competency at that turn. It returns the snapshots.
func (s *Service) Rescore(ctx context.Context, iv *domain.Interview, turn *domain.Turn) ([]domain.ConfidenceSnapshot, error) {
	ledger, err := s.Evidence.All(ctx, iv.ID)
	if err != nil {
		return nil, err
	}
	if len(ledger) == 0 {
		return nil, nil
	}

	// Assign stable short ids the model can reference in revisions.
	idMap, ledgerText := renderLedger(ledger)

	user := fmt.Sprintf(rescoreUserTmpl, iv.Level, iv.Type, turn.Turn, turn.Question, turn.Answer, ledgerText)

	var res rescoreResult
	if err := llm.CallJSON(ctx, s.LLM.Caller, rescoreSystem, user, &res); err != nil {
		return nil, fmt.Errorf("rescore: %w", err)
	}

	// Apply retroactive evidence revisions.
	now := time.Now().UTC()
	for _, rev := range res.Revisions {
		item, ok := idMap[rev.EvidenceID]
		if !ok || rev.NewStrength == item.Strength {
			continue
		}
		if err := s.Evidence.ApplyRevision(ctx, item.ID, rev.NewStrength, domain.Revision{
			AtTurn:      turn.Turn,
			OldStrength: item.Strength,
			NewStrength: rev.NewStrength,
			Note:        rev.Note,
			At:          now,
		}); err != nil {
			log.Printf("[CONFIDENCE] apply revision %s: %v", rev.EvidenceID, err)
		}
	}

	// Persist one snapshot per competency at this turn.
	snapshots := make([]domain.ConfidenceSnapshot, 0, len(res.Competencies))
	docs := make([]any, 0, len(res.Competencies))
	for _, c := range res.Competencies {
		turns := dedupeInts(c.EvidenceTurns)
		snap := domain.ConfidenceSnapshot{
			InterviewID:   iv.ID,
			Competency:    c.Name,
			Turn:          turn.Turn,
			Confidence:    clamp01(c.Confidence),
			Cool:          clamp01(c.Cool),
			Normal:        clamp01(c.Normal),
			Hot:           clamp01(c.Hot),
			EvidenceCount: countForCompetency(ledger, c.Name),
			EvidenceTurns: turns,
			Rationale:     c.Rationale,
			CreatedAt:     now,
		}
		snapshots = append(snapshots, snap)
		docs = append(docs, snap)
	}
	if len(docs) > 0 {
		if _, err := s.Store.Coll(store.CollConfidenceScores).InsertMany(ctx, docs); err != nil {
			return nil, fmt.Errorf("persist snapshots: %w", err)
		}
	}
	return snapshots, nil
}

// taggedItem pairs an evidence item with the short id shown to the model.
type taggedItem struct {
	id   string
	item domain.EvidenceItem
}

func renderLedger(ledger []domain.EvidenceItem) (map[string]domain.EvidenceItem, string) {
	idMap := make(map[string]domain.EvidenceItem, len(ledger))
	byComp := map[string][]taggedItem{}
	for i := range ledger {
		id := fmt.Sprintf("e%d", i+1)
		idMap[id] = ledger[i]
		byComp[ledger[i].Competency] = append(byComp[ledger[i].Competency], taggedItem{id: id, item: ledger[i]})
	}

	// Stable ordering for reproducibility.
	comps := make([]string, 0, len(byComp))
	for c := range byComp {
		comps = append(comps, c)
	}
	sort.Strings(comps)

	var b []byte
	for _, c := range comps {
		b = append(b, fmt.Sprintf("\n## %s\n", c)...)
		for _, ti := range byComp[c] {
			it := ti.item
			b = append(b, fmt.Sprintf("- [%s] turn %d | %s | strength %.2f | concepts %v | %q -> %s\n",
				ti.id, it.Turn, it.Polarity, it.Strength, it.Concepts, it.SupportingQuote, it.Interpretation)...)
		}
	}
	return idMap, string(b)
}

func countForCompetency(ledger []domain.EvidenceItem, comp string) int {
	n := 0
	for _, it := range ledger {
		if it.Competency == comp {
			n++
		}
	}
	return n
}

func dedupeInts(in []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
