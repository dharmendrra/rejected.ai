package learning

import (
	"testing"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func iv(at time.Time, scores ...domain.CompetencyScore) ScoredInterview {
	return ScoredInterview{InterviewID: bson.NewObjectID(), At: at, Scores: scores}
}

func sc(comp string, normal, conf float64) domain.CompetencyScore {
	return domain.CompetencyScore{Competency: comp, Normal: normal, Confidence: conf}
}

func TestBuildTrends_DirectionsAndOrdering(t *testing.T) {
	cand := bson.NewObjectID()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.AddDate(0, 1, 0)
	t2 := t0.AddDate(0, 2, 0)

	// Feed interviews out of chronological order to prove they get sorted.
	trends := BuildTrends(cand, []ScoredInterview{
		iv(t2, sc("architecture", 0.9, 0.8), sc("delivery", 0.4, 0.7), sc("comms", 0.6, 0.6)),
		iv(t0, sc("architecture", 0.5, 0.6), sc("delivery", 0.8, 0.7), sc("comms", 0.6, 0.5)),
		iv(t1, sc("architecture", 0.7, 0.7)), // delivery/comms absent this interview
	}, t0)

	got := map[string]domain.HistoricalTrend{}
	for _, tr := range trends {
		got[tr.Competency] = tr
	}

	// architecture: 0.5 -> 0.7 -> 0.9, improving, 3 points oldest-first.
	arch := got["architecture"]
	if arch.Direction != domain.TrendImproving {
		t.Errorf("architecture direction = %q, want improving", arch.Direction)
	}
	if arch.Interviews != 3 || arch.First != 0.5 || arch.Latest != 0.9 {
		t.Errorf("architecture trajectory wrong: %+v", arch)
	}
	if !arch.Points[0].At.Before(arch.Points[1].At) || !arch.Points[1].At.Before(arch.Points[2].At) {
		t.Errorf("architecture points not oldest-first: %+v", arch.Points)
	}

	// delivery: 0.8 -> 0.4, declining.
	if d := got["delivery"]; d.Direction != domain.TrendDeclining || d.Interviews != 2 {
		t.Errorf("delivery = %+v, want declining with 2 points", d)
	}
	// comms: 0.6 -> 0.6, stable.
	if c := got["comms"]; c.Direction != domain.TrendStable {
		t.Errorf("comms direction = %q, want stable", c.Direction)
	}
}

func TestBuildTrends_SinglePointIsNew(t *testing.T) {
	cand := bson.NewObjectID()
	trends := BuildTrends(cand, []ScoredInterview{
		iv(time.Now(), sc("architecture", 0.7, 0.6)),
	}, time.Now())
	if len(trends) != 1 || trends[0].Direction != domain.TrendNew {
		t.Fatalf("want one 'new' trend, got %+v", trends)
	}
}

func TestBuildTrends_EpsilonNoise(t *testing.T) {
	cand := bson.NewObjectID()
	t0 := time.Now()
	// 0.50 -> 0.53 is within epsilon (0.05): stable, not improving.
	trends := BuildTrends(cand, []ScoredInterview{
		iv(t0, sc("delivery", 0.50, 0.6)),
		iv(t0.Add(time.Hour), sc("delivery", 0.53, 0.6)),
	}, t0)
	if trends[0].Direction != domain.TrendStable {
		t.Errorf("within-epsilon move = %q, want stable", trends[0].Direction)
	}
}

func TestSummarize(t *testing.T) {
	trends := []domain.HistoricalTrend{
		{Competency: "architecture", Direction: domain.TrendImproving},
		{Competency: "comms", Direction: domain.TrendStable},
		{Competency: "delivery", Direction: domain.TrendDeclining},
		{Competency: "ai", Direction: domain.TrendNew},
	}
	up, down, stable := Summarize(trends)
	if len(up) != 1 || up[0] != "architecture" {
		t.Errorf("improving = %v", up)
	}
	if len(down) != 1 || down[0] != "delivery" {
		t.Errorf("declining = %v", down)
	}
	if len(stable) != 1 || stable[0] != "comms" {
		t.Errorf("stable = %v", stable)
	}
}
