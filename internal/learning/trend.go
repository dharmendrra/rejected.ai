// Package learning derives cross-interview trends: how a candidate's measured
// competency scores move across their own interviews over time. It is purely
// deterministic — arithmetic over previously stored CompetencyScores — and never
// infers any trait or calls an LLM.
package learning

import (
	"sort"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// trendEpsilon is the smallest change in the 0..1 Normal score that counts as a
// real move rather than noise. Deltas within ±epsilon are reported as stable.
const trendEpsilon = 0.05

// ScoredInterview pairs one interview's identity and time with its finalized
// competency scores. It is the only input BuildTrends needs.
type ScoredInterview struct {
	InterviewID bson.ObjectID
	At          time.Time
	Scores      []domain.CompetencyScore
}

// BuildTrends groups competency scores across a candidate's interviews and
// computes each competency's trajectory. Interviews may arrive in any order;
// points are sorted oldest-first per competency before deltas are computed.
// Competencies are returned in stable alphabetical order for deterministic output.
func BuildTrends(candidateID bson.ObjectID, interviews []ScoredInterview, now time.Time) []domain.HistoricalTrend {
	byComp := map[string][]domain.TrendPoint{}
	for _, iv := range interviews {
		for _, sc := range iv.Scores {
			byComp[sc.Competency] = append(byComp[sc.Competency], domain.TrendPoint{
				InterviewID: iv.InterviewID,
				Normal:      sc.Normal,
				Confidence:  sc.Confidence,
				At:          iv.At,
			})
		}
	}

	comps := make([]string, 0, len(byComp))
	for c := range byComp {
		comps = append(comps, c)
	}
	sort.Strings(comps)

	trends := make([]domain.HistoricalTrend, 0, len(comps))
	for _, comp := range comps {
		pts := byComp[comp]
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })

		first := pts[0].Normal
		latest := pts[len(pts)-1].Normal
		delta := latest - first

		trends = append(trends, domain.HistoricalTrend{
			CandidateID: candidateID,
			Competency:  comp,
			Points:      pts,
			Interviews:  len(pts),
			First:       first,
			Latest:      latest,
			Delta:       delta,
			Direction:   direction(len(pts), delta),
			CreatedAt:   now,
		})
	}
	return trends
}

// direction classifies a competency's movement from its delta. A single data
// point has no trajectory yet.
func direction(points int, delta float64) string {
	if points < 2 {
		return domain.TrendNew
	}
	switch {
	case delta > trendEpsilon:
		return domain.TrendImproving
	case delta < -trendEpsilon:
		return domain.TrendDeclining
	default:
		return domain.TrendStable
	}
}

// Summarize buckets competency names by direction for an at-a-glance view of a
// candidate's patterns. Each slice preserves the input (alphabetical) order.
func Summarize(trends []domain.HistoricalTrend) (improving, declining, stable []string) {
	for _, t := range trends {
		switch t.Direction {
		case domain.TrendImproving:
			improving = append(improving, t.Competency)
		case domain.TrendDeclining:
			declining = append(declining, t.Competency)
		case domain.TrendStable:
			stable = append(stable, t.Competency)
		}
	}
	return
}
