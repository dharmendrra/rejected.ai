package dashboard

import (
	"testing"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// helpers ---------------------------------------------------------------------

func mkInterview(at time.Time, typ, level string, rigor int) domain.Interview {
	return domain.Interview{
		ID:           bson.NewObjectID(),
		Type:         typ,
		Level:        level,
		RigorPercent: rigor,
		CreatedAt:    at,
		UpdatedAt:    at,
	}
}

func mkRec(ivID bson.ObjectID, decision string, conf float64) domain.Recommendation {
	return domain.Recommendation{
		InterviewID:     ivID,
		Decision:        decision,
		ConfidenceLevel: conf,
	}
}

func mkScore(ivID bson.ObjectID, comp string, normal, conf float64) domain.CompetencyScore {
	return domain.CompetencyScore{
		InterviewID: ivID,
		Competency:  comp,
		Normal:      normal,
		Confidence:  conf,
		Cool:        normal - 0.1,
		Hot:         normal + 0.1,
	}
}

func verdictCount(resp Response, decision string) int {
	for _, v := range resp.VerdictMix {
		if v.Decision == decision {
			return v.Count
		}
	}
	return -1
}

// tests -----------------------------------------------------------------------

func TestAggregate_VerdictMixAndKPIs(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	iv1 := mkInterview(now, "System Design", "Senior Engineer", 70)
	iv2 := mkInterview(now.Add(time.Hour), "System Design", "Senior Engineer", 80)
	iv3 := mkInterview(now.Add(2*time.Hour), "Behavioral", "Staff Engineer", 50)
	// iv3 has NO recommendation -> pending report, excluded from verdict/confidence.

	in := Input{
		Interviews: []domain.Interview{iv1, iv2, iv3},
		Recs: []domain.Recommendation{
			mkRec(iv1.ID, domain.DecisionStrongHire, 0.9),
			mkRec(iv2.ID, domain.DecisionHire, 0.7),
		},
		Turns: []domain.Turn{
			{InterviewID: iv1.ID, Answered: true},
			{InterviewID: iv1.ID, Answered: false},
			{InterviewID: iv2.ID, Answered: true},
		},
		CandidateName: map[bson.ObjectID]string{
			iv1.ID: "Alice Smith",
			iv2.ID: "Alice Smith", // same person (different objectID would normally appear)
			iv3.ID: "Bob Jones",
		},
		JobTitle: map[bson.ObjectID]string{},
	}

	resp := Aggregate(in, Scope{Candidate: "all"}, now)

	if resp.KPIs.TotalInterviews != 3 {
		t.Errorf("total_interviews = %d, want 3", resp.KPIs.TotalInterviews)
	}
	if resp.KPIs.CompletedReports != 2 {
		t.Errorf("completed_reports = %d, want 2", resp.KPIs.CompletedReports)
	}
	if resp.KPIs.PendingReports != 1 {
		t.Errorf("pending_reports = %d, want 1", resp.KPIs.PendingReports)
	}
	if resp.KPIs.QuestionsAsked != 3 {
		t.Errorf("questions_asked = %d, want 3", resp.KPIs.QuestionsAsked)
	}
	if resp.KPIs.QuestionsAnswered != 2 {
		t.Errorf("questions_answered = %d, want 2", resp.KPIs.QuestionsAnswered)
	}
	// avg over completed: (0.9 + 0.7) / 2 = 0.8
	if got := resp.KPIs.AvgConfidence; got < 0.7999 || got > 0.8001 {
		t.Errorf("avg_confidence = %f, want 0.8", got)
	}
	if resp.KPIs.Candidates != 2 {
		t.Errorf("candidates = %d, want 2 (Alice, Bob)", resp.KPIs.Candidates)
	}

	// Verdict mix: all 5 decisions present, in locked order.
	if len(resp.VerdictMix) != 5 {
		t.Fatalf("verdict_mix len = %d, want 5", len(resp.VerdictMix))
	}
	wantOrder := []string{
		domain.DecisionStrongHire, domain.DecisionHire, domain.DecisionHireWithRisks,
		domain.DecisionBorderline, domain.DecisionNoHire,
	}
	for i, d := range wantOrder {
		if resp.VerdictMix[i].Decision != d {
			t.Errorf("verdict_mix[%d] = %q, want %q", i, resp.VerdictMix[i].Decision, d)
		}
	}
	if c := verdictCount(resp, domain.DecisionStrongHire); c != 1 {
		t.Errorf("strong_hire count = %d, want 1", c)
	}
	if c := verdictCount(resp, domain.DecisionHire); c != 1 {
		t.Errorf("hire count = %d, want 1", c)
	}
	if c := verdictCount(resp, domain.DecisionNoHire); c != 0 {
		t.Errorf("no_hire count = %d, want 0", c)
	}

	// confidence_over_time only includes completed interviews, time-ordered.
	if len(resp.ConfidenceOverTime) != 2 {
		t.Fatalf("confidence_over_time len = %d, want 2", len(resp.ConfidenceOverTime))
	}
	if !resp.ConfidenceOverTime[0].At.Before(resp.ConfidenceOverTime[1].At) {
		t.Errorf("confidence_over_time not time-ordered")
	}
	if resp.ConfidenceOverTime[0].CandidateName != "Alice Smith" {
		t.Errorf("confidence point candidate = %q", resp.ConfidenceOverTime[0].CandidateName)
	}

	// rigor_vs_confidence only completed interviews.
	if len(resp.RigorVsConfidence) != 2 {
		t.Errorf("rigor_vs_confidence len = %d, want 2", len(resp.RigorVsConfidence))
	}

	// Coverage by type: System Design=2, Behavioral=1 (count desc).
	if len(resp.Coverage.ByType) != 2 || resp.Coverage.ByType[0].Key != "System Design" || resp.Coverage.ByType[0].Count != 2 {
		t.Errorf("coverage by_type wrong: %+v", resp.Coverage.ByType)
	}
}

func TestAggregate_TrendsByCandidateName(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Alice: two interviews, architecture improves 0.5 -> 0.9.
	a1 := mkInterview(now, "System Design", "Senior", 70)
	a2 := mkInterview(now.Add(24*time.Hour), "System Design", "Senior", 70)
	// Bob: single interview -> "new" trend, not eligible for most-improved.
	b1 := mkInterview(now.Add(48*time.Hour), "System Design", "Senior", 70)

	in := Input{
		Interviews: []domain.Interview{a2, a1, b1}, // unsorted on purpose
		Recs: []domain.Recommendation{
			mkRec(a1.ID, domain.DecisionHire, 0.6),
			mkRec(a2.ID, domain.DecisionStrongHire, 0.9),
			mkRec(b1.ID, domain.DecisionHire, 0.7),
		},
		Competency: []domain.CompetencyScore{
			mkScore(a1.ID, "architecture", 0.5, 0.6),
			mkScore(a1.ID, "delivery", 0.8, 0.6),
			mkScore(a2.ID, "architecture", 0.9, 0.8),
			mkScore(a2.ID, "delivery", 0.5, 0.7), // declines
			mkScore(b1.ID, "architecture", 0.7, 0.7),
		},
		// Name normalization: "Alice " vs "alice" should be the same candidate.
		CandidateName: map[bson.ObjectID]string{
			a1.ID: "Alice ",
			a2.ID: "alice",
			b1.ID: "Bob",
		},
		JobTitle: map[bson.ObjectID]string{},
	}

	resp := Aggregate(in, Scope{Candidate: "all"}, now)

	// Find Alice's architecture trend (improving). Because Bob also has
	// "architecture" as a single point, there are two architecture trends.
	var aliceArchFound, aliceDeliveryFound, bobArchNew bool
	for _, tr := range resp.CompetencyTrends {
		switch {
		case tr.Competency == "architecture" && tr.Direction == domain.TrendImproving:
			aliceArchFound = true
			if tr.First != 0.5 || tr.Latest != 0.9 {
				t.Errorf("alice architecture first/latest = %f/%f, want 0.5/0.9", tr.First, tr.Latest)
			}
			if len(tr.Points) != 2 {
				t.Errorf("alice architecture points = %d, want 2", len(tr.Points))
			}
		case tr.Competency == "delivery" && tr.Direction == domain.TrendDeclining:
			aliceDeliveryFound = true
		case tr.Competency == "architecture" && tr.Direction == domain.TrendNew:
			bobArchNew = true
		}
	}
	if !aliceArchFound {
		t.Error("expected Alice's architecture improving trend")
	}
	if !aliceDeliveryFound {
		t.Error("expected Alice's delivery declining trend")
	}
	if !bobArchNew {
		t.Error("expected Bob's architecture 'new' trend (single interview)")
	}

	// Most improved = architecture (delta +0.4), the only >=2 interview gain.
	if resp.KPIs.MostImprovedCompetency != "architecture" {
		t.Errorf("most_improved_competency = %q, want architecture", resp.KPIs.MostImprovedCompetency)
	}

	// Two distinct candidate names.
	if resp.KPIs.Candidates != 2 {
		t.Errorf("candidates = %d, want 2", resp.KPIs.Candidates)
	}
}

func TestAggregate_MostImprovedEmptyWhenSingleInterview(t *testing.T) {
	now := time.Now().UTC()
	iv := mkInterview(now, "System Design", "Senior", 70)
	in := Input{
		Interviews:    []domain.Interview{iv},
		Recs:          []domain.Recommendation{mkRec(iv.ID, domain.DecisionHire, 0.7)},
		Competency:    []domain.CompetencyScore{mkScore(iv.ID, "architecture", 0.7, 0.6)},
		CandidateName: map[bson.ObjectID]string{iv.ID: "Solo"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)
	if resp.KPIs.MostImprovedCompetency != "" {
		t.Errorf("most_improved should be empty with <2 interviews, got %q", resp.KPIs.MostImprovedCompetency)
	}
}

func TestAggregate_CompetencyProfileFirstVsLatest(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	iv1 := mkInterview(now, "System Design", "Senior", 70)
	iv2 := mkInterview(now.Add(time.Hour), "System Design", "Senior", 70)
	in := Input{
		Interviews: []domain.Interview{iv2, iv1},
		Recs: []domain.Recommendation{
			mkRec(iv1.ID, domain.DecisionHire, 0.6),
			mkRec(iv2.ID, domain.DecisionHire, 0.7),
		},
		Competency: []domain.CompetencyScore{
			mkScore(iv1.ID, "architecture", 0.4, 0.5),
			mkScore(iv2.ID, "architecture", 0.8, 0.9),
		},
		CandidateName: map[bson.ObjectID]string{iv1.ID: "A", iv2.ID: "A"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)
	if len(resp.CompetencyProfile) != 1 {
		t.Fatalf("competency_profile len = %d, want 1", len(resp.CompetencyProfile))
	}
	p := resp.CompetencyProfile[0]
	if p.FirstNormal != 0.4 {
		t.Errorf("first_normal = %f, want 0.4 (earliest)", p.FirstNormal)
	}
	if p.Normal != 0.8 {
		t.Errorf("normal = %f, want 0.8 (latest)", p.Normal)
	}
	if p.Confidence != 0.9 {
		t.Errorf("confidence = %f, want 0.9 (latest)", p.Confidence)
	}
}

func TestAggregate_RiskBucketing(t *testing.T) {
	now := time.Now().UTC()
	iv := mkInterview(now, "System Design", "Senior", 70)
	in := Input{
		Interviews: []domain.Interview{iv},
		Recs:       []domain.Recommendation{mkRec(iv.ID, domain.DecisionHire, 0.7)},
		Risks: []domain.RiskDoc{
			{InterviewID: iv.ID, Risks: []domain.RiskItem{
				{Category: domain.RiskMissing, Severity: "high"},
				{Category: domain.RiskMissing, Severity: "high"},
				{Category: domain.RiskMissing, Severity: "low"},
				{Category: domain.RiskWeak, Severity: "medium"},
				{Category: domain.RiskJD, Severity: "high"},
			}},
		},
		CandidateName: map[bson.ObjectID]string{iv.ID: "A"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)

	got := map[string]int{}
	for _, b := range resp.Risks {
		got[b.Category+"/"+b.Severity] = b.Count
	}
	if got["missing/high"] != 2 {
		t.Errorf("missing/high = %d, want 2", got["missing/high"])
	}
	if got["missing/low"] != 1 {
		t.Errorf("missing/low = %d, want 1", got["missing/low"])
	}
	if got["weak/medium"] != 1 {
		t.Errorf("weak/medium = %d, want 1", got["weak/medium"])
	}
	if got["jd_risk/high"] != 1 {
		t.Errorf("jd_risk/high = %d, want 1", got["jd_risk/high"])
	}

	// Ordering: category (missing, weak, jd_risk) then severity (low, medium, high).
	if len(resp.Risks) < 2 {
		t.Fatalf("expected multiple risk buckets")
	}
	if resp.Risks[0].Category != domain.RiskMissing {
		t.Errorf("first bucket category = %q, want missing", resp.Risks[0].Category)
	}
	// Within missing: low before high.
	if resp.Risks[0].Severity != "low" {
		t.Errorf("first missing bucket severity = %q, want low (severity order)", resp.Risks[0].Severity)
	}
}

func TestAggregate_SignalFrequencyOrdering(t *testing.T) {
	now := time.Now().UTC()
	iv1 := mkInterview(now, "System Design", "Senior", 70)
	iv2 := mkInterview(now.Add(time.Hour), "System Design", "Senior", 70)
	in := Input{
		Interviews: []domain.Interview{iv1, iv2},
		Recs: []domain.Recommendation{
			mkRec(iv1.ID, domain.DecisionHire, 0.7),
			mkRec(iv2.ID, domain.DecisionHire, 0.7),
		},
		Signals: []domain.SignalsDoc{
			{InterviewID: iv1.ID, Signals: []domain.StrongestSignal{
				{Name: "Clear communication"},
				{Name: "Systems thinking"},
			}},
			{InterviewID: iv2.ID, Signals: []domain.StrongestSignal{
				{Name: "Clear communication"},
				{Name: "Pragmatism"},
				{Name: ""}, // blank ignored
			}},
		},
		CandidateName: map[bson.ObjectID]string{iv1.ID: "A", iv2.ID: "A"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)

	if len(resp.TopSignals) != 3 {
		t.Fatalf("top_signals len = %d, want 3 (blank dropped): %+v", len(resp.TopSignals), resp.TopSignals)
	}
	// Highest frequency first.
	if resp.TopSignals[0].Name != "Clear communication" || resp.TopSignals[0].Count != 2 {
		t.Errorf("top signal = %+v, want Clear communication x2", resp.TopSignals[0])
	}
	// Ties broken alphabetically: Pragmatism before Systems thinking.
	if resp.TopSignals[1].Name != "Pragmatism" || resp.TopSignals[2].Name != "Systems thinking" {
		t.Errorf("tie ordering wrong: %+v", resp.TopSignals)
	}
}

func TestAggregate_PersonaCompetencyAverages(t *testing.T) {
	now := time.Now().UTC()
	iv1 := mkInterview(now, "System Design", "Senior", 70)
	iv2 := mkInterview(now.Add(time.Hour), "System Design", "Senior", 70)
	in := Input{
		Interviews: []domain.Interview{iv1, iv2},
		Recs: []domain.Recommendation{
			{InterviewID: iv1.ID, Decision: domain.DecisionHire, ConfidenceLevel: 0.7, Personas: []domain.PersonaView{
				{Persona: "System Architect", PerCompetency: []domain.PersonaCompetency{
					{Competency: "architecture", Score: 0.6},
				}},
			}},
			{InterviewID: iv2.ID, Decision: domain.DecisionHire, ConfidenceLevel: 0.7, Personas: []domain.PersonaView{
				{Persona: "System Architect", PerCompetency: []domain.PersonaCompetency{
					{Competency: "architecture", Score: 0.8},
				}},
			}},
		},
		CandidateName: map[bson.ObjectID]string{iv1.ID: "A", iv2.ID: "A"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)
	if len(resp.PersonaCompetency) != 1 {
		t.Fatalf("persona_competency len = %d, want 1", len(resp.PersonaCompetency))
	}
	pc := resp.PersonaCompetency[0]
	if pc.Persona != "System Architect" || len(pc.Competencies) != 1 {
		t.Fatalf("persona wrong: %+v", pc)
	}
	// avg of 0.6 and 0.8 = 0.7
	if got := pc.Competencies[0].AvgScore; got < 0.6999 || got > 0.7001 {
		t.Errorf("avg_score = %f, want 0.7", got)
	}
}

func TestAggregate_ScoreEvolutionAvgNormalByTurn(t *testing.T) {
	now := time.Now().UTC()
	iv := mkInterview(now, "System Design", "Senior", 70)
	in := Input{
		Interviews: []domain.Interview{iv},
		Recs:       []domain.Recommendation{mkRec(iv.ID, domain.DecisionHire, 0.7)},
		Snapshots: []domain.ConfidenceSnapshot{
			{InterviewID: iv.ID, Turn: 1, Competency: "architecture", Normal: 0.4},
			{InterviewID: iv.ID, Turn: 1, Competency: "delivery", Normal: 0.6},
			{InterviewID: iv.ID, Turn: 2, Competency: "architecture", Normal: 0.8},
		},
		CandidateName: map[bson.ObjectID]string{iv.ID: "A"},
		JobTitle:      map[bson.ObjectID]string{},
	}
	resp := Aggregate(in, Scope{Candidate: "all"}, now)
	if len(resp.ScoreEvolution) != 1 {
		t.Fatalf("score_evolution len = %d, want 1", len(resp.ScoreEvolution))
	}
	se := resp.ScoreEvolution[0]
	if len(se.Series) != 2 {
		t.Fatalf("series len = %d, want 2 turns", len(se.Series))
	}
	// Turn 1: mean(0.4, 0.6) = 0.5
	if se.Series[0].Turn != 1 || se.Series[0].AvgNormal < 0.4999 || se.Series[0].AvgNormal > 0.5001 {
		t.Errorf("turn 1 avg = %+v, want 0.5", se.Series[0])
	}
	// Turn 2: mean(0.8) = 0.8
	if se.Series[1].Turn != 2 || se.Series[1].AvgNormal < 0.7999 || se.Series[1].AvgNormal > 0.8001 {
		t.Errorf("turn 2 avg = %+v, want 0.8", se.Series[1])
	}
}

func TestAggregate_EmptyInputProducesStableShape(t *testing.T) {
	now := time.Now().UTC()
	resp := Aggregate(Input{
		CandidateName: map[bson.ObjectID]string{},
		JobTitle:      map[bson.ObjectID]string{},
	}, Scope{Candidate: "all"}, now)

	if resp.KPIs.TotalInterviews != 0 {
		t.Errorf("total_interviews = %d, want 0", resp.KPIs.TotalInterviews)
	}
	if resp.KPIs.AvgConfidence != 0 {
		t.Errorf("avg_confidence = %f, want 0", resp.KPIs.AvgConfidence)
	}
	// verdict_mix still lists all 5 decisions with count 0.
	if len(resp.VerdictMix) != 5 {
		t.Errorf("verdict_mix len = %d, want 5 even when empty", len(resp.VerdictMix))
	}
	for _, v := range resp.VerdictMix {
		if v.Count != 0 {
			t.Errorf("empty verdict count for %q = %d, want 0", v.Decision, v.Count)
		}
	}
}
