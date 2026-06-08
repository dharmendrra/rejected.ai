// Package dashboard aggregates everything the platform already stores across
// interviews — verdicts, confidence, competencies, risks, signals, evaluator
// personas, and trends — into a single portfolio view for the Progress
// Dashboard screen.
//
// The aggregator is intentionally PURE: it takes already-loaded domain
// documents (no DB access, no LLM, no HTTP) and returns the response struct.
// That keeps it deterministic and table-testable. The HTTP handler in
// internal/api does the batched Mongo reads, then calls Aggregate.
package dashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/learning"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ─── Response payload ─────────────────────────────────────────────────────────

// Response is the full dashboard payload returned by GET /api/dashboard.
type Response struct {
	GeneratedAt        time.Time           `json:"generated_at"`
	Scope              Scope               `json:"scope"`
	KPIs               KPIs                `json:"kpis"`
	VerdictMix         []VerdictCount      `json:"verdict_mix"`
	ConfidenceOverTime []ConfidencePoint   `json:"confidence_over_time"`
	CompetencyTrends   []CompetencyTrend   `json:"competency_trends"`
	CompetencyProfile  []CompetencyProfile `json:"competency_profile"`
	RigorVsConfidence  []RigorPoint        `json:"rigor_vs_confidence"`
	Coverage           Coverage            `json:"coverage"`
	Risks              []RiskBucket        `json:"risks"`
	TopSignals         []SignalCount       `json:"top_signals"`
	PersonaCompetency  []PersonaCompetency `json:"persona_competency"`
	ScoreEvolution     []ScoreEvolution    `json:"score_evolution"`
}

// Scope echoes the query filters applied to the dataset.
type Scope struct {
	Candidate string     `json:"candidate"`
	From      *time.Time `json:"from"`
	To        *time.Time `json:"to"`
}

// KPIs are the headline stat tiles.
type KPIs struct {
	TotalInterviews        int     `json:"total_interviews"`
	CompletedReports       int     `json:"completed_reports"`
	PendingReports         int     `json:"pending_reports"`
	QuestionsAsked         int     `json:"questions_asked"`
	QuestionsAnswered      int     `json:"questions_answered"`
	AvgConfidence          float64 `json:"avg_confidence"`
	MostImprovedCompetency string  `json:"most_improved_competency"`
	Candidates             int     `json:"candidates"`
}

// VerdictCount is one decision label and how many completed reports carry it.
type VerdictCount struct {
	Decision string `json:"decision"`
	Count    int    `json:"count"`
}

// ConfidencePoint is one completed interview on the confidence-over-time line.
type ConfidencePoint struct {
	InterviewID   string    `json:"interview_id"`
	At            time.Time `json:"at"`
	Confidence    float64   `json:"confidence"`
	Decision      string    `json:"decision"`
	Level         string    `json:"level"`
	Type          string    `json:"type"`
	RigorPercent  int       `json:"rigor_percent"`
	CandidateName string    `json:"candidate_name"`
	JobTitle      string    `json:"job_title"`
}

// CompetencyTrend is one competency's cross-interview trajectory (per candidate name).
type CompetencyTrend struct {
	Competency string       `json:"competency"`
	Direction  string       `json:"direction"`
	First      float64      `json:"first"`
	Latest     float64      `json:"latest"`
	Delta      float64      `json:"delta"`
	Points     []TrendPoint `json:"points"`
}

// TrendPoint is one competency measurement at one interview.
type TrendPoint struct {
	InterviewID string    `json:"interview_id"`
	At          time.Time `json:"at"`
	Normal      float64   `json:"normal"`
	Confidence  float64   `json:"confidence"`
}

// CompetencyProfile is the latest score per competency, with the earliest
// Normal value for a first-vs-latest radar overlay.
type CompetencyProfile struct {
	Competency  string  `json:"competency"`
	Cool        float64 `json:"cool"`
	Normal      float64 `json:"normal"`
	Hot         float64 `json:"hot"`
	Confidence  float64 `json:"confidence"`
	FirstNormal float64 `json:"first_normal"`
}

// RigorPoint is one completed interview on the rigor-vs-confidence scatter.
type RigorPoint struct {
	InterviewID  string  `json:"interview_id"`
	RigorPercent int     `json:"rigor_percent"`
	Confidence   float64 `json:"confidence"`
	Decision     string  `json:"decision"`
	Type         string  `json:"type"`
}

// Coverage counts interviews by type and by level.
type Coverage struct {
	ByType  []KeyCount `json:"by_type"`
	ByLevel []KeyCount `json:"by_level"`
}

// KeyCount is a generic label/count pair.
type KeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// RiskBucket is one category×severity total across all risk docs.
type RiskBucket struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

// SignalCount is one strongest-signal name and its frequency.
type SignalCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// PersonaCompetency is one evaluator persona's average score per competency.
type PersonaCompetency struct {
	Persona      string                 `json:"persona"`
	Competencies []PersonaCompetencyAvg `json:"competencies"`
}

// PersonaCompetencyAvg is one competency's mean score for a persona.
type PersonaCompetencyAvg struct {
	Competency string  `json:"competency"`
	AvgScore   float64 `json:"avg_score"`
}

// ScoreEvolution is one interview's within-interview normal-by-turn line.
type ScoreEvolution struct {
	InterviewID   string          `json:"interview_id"`
	CandidateName string          `json:"candidate_name"`
	Type          string          `json:"type"`
	Series        []ScoreEvoPoint `json:"series"`
}

// ScoreEvoPoint is the mean Normal across competencies at one turn.
type ScoreEvoPoint struct {
	Turn      int     `json:"turn"`
	AvgNormal float64 `json:"avg_normal"`
}

// ─── Input ────────────────────────────────────────────────────────────────────

// Input bundles the already-loaded domain documents the aggregator needs. The
// handler populates it from batched Mongo reads. Lookups map an interview ID to
// the candidate name / job title to use for labels (resolved by the handler
// since candidate_profile_id / job_description_id are not on every doc here).
type Input struct {
	Interviews    []domain.Interview
	Recs          []domain.Recommendation
	Competency    []domain.CompetencyScore
	Risks         []domain.RiskDoc
	Signals       []domain.SignalsDoc
	Snapshots     []domain.ConfidenceSnapshot
	Turns         []domain.Turn
	CandidateName map[bson.ObjectID]string // interview ID -> candidate name
	JobTitle      map[bson.ObjectID]string // interview ID -> job title
}

// decisionOrder is the fixed, locked order of verdict labels (see AGENTS.md).
var decisionOrder = []string{
	domain.DecisionStrongHire,
	domain.DecisionHire,
	domain.DecisionHireWithRisks,
	domain.DecisionBorderline,
	domain.DecisionNoHire,
}

// riskCategoryOrder and riskSeverityOrder give risk buckets a deterministic order.
var riskCategoryOrder = []string{domain.RiskMissing, domain.RiskWeak, domain.RiskJD}
var riskSeverityOrder = []string{"low", "medium", "high"}

// normalizeName trims and lowercases a candidate name for best-effort identity.
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Aggregate turns already-loaded documents into the dashboard response.
//
// An interview is "completed" (has a generated report) iff it has a
// recommendation. Interviews without a recommendation are excluded from
// verdict/confidence/competency charts and counted under pending_reports.
func Aggregate(in Input, scope Scope, now time.Time) Response {
	resp := Response{
		GeneratedAt: now,
		Scope:       scope,
	}

	// Index recommendations and competency scores by interview ID.
	recByIv := make(map[bson.ObjectID]domain.Recommendation, len(in.Recs))
	for _, rec := range in.Recs {
		recByIv[rec.InterviewID] = rec
	}
	compByIv := make(map[bson.ObjectID][]domain.CompetencyScore)
	for _, cs := range in.Competency {
		compByIv[cs.InterviewID] = append(compByIv[cs.InterviewID], cs)
	}

	// Interviews oldest-first so time-ordered series are chronological.
	interviews := append([]domain.Interview(nil), in.Interviews...)
	sort.SliceStable(interviews, func(i, j int) bool {
		return interviews[i].CreatedAt.Before(interviews[j].CreatedAt)
	})

	// ── KPIs, coverage, verdict mix, confidence/rigor series ──────────────────
	verdictCounts := map[string]int{}
	byType := map[string]int{}
	byLevel := map[string]int{}
	candidateNames := map[string]struct{}{}

	var confSum float64
	var confN int

	for _, iv := range interviews {
		resp.KPIs.TotalInterviews++
		if t := strings.TrimSpace(iv.Type); t != "" {
			byType[t]++
		}
		if l := strings.TrimSpace(iv.Level); l != "" {
			byLevel[l]++
		}
		name := in.CandidateName[iv.ID]
		if norm := normalizeName(name); norm != "" {
			candidateNames[norm] = struct{}{}
		}

		rec, completed := recByIv[iv.ID]
		if !completed {
			resp.KPIs.PendingReports++
			continue
		}
		resp.KPIs.CompletedReports++
		verdictCounts[rec.Decision]++
		confSum += rec.ConfidenceLevel
		confN++

		resp.ConfidenceOverTime = append(resp.ConfidenceOverTime, ConfidencePoint{
			InterviewID:   iv.ID.Hex(),
			At:            iv.CreatedAt,
			Confidence:    rec.ConfidenceLevel,
			Decision:      rec.Decision,
			Level:         iv.Level,
			Type:          iv.Type,
			RigorPercent:  iv.RigorPercent,
			CandidateName: name,
			JobTitle:      in.JobTitle[iv.ID],
		})
		resp.RigorVsConfidence = append(resp.RigorVsConfidence, RigorPoint{
			InterviewID:  iv.ID.Hex(),
			RigorPercent: iv.RigorPercent,
			Confidence:   rec.ConfidenceLevel,
			Decision:     rec.Decision,
			Type:         iv.Type,
		})
	}

	// Questions asked / answered across all interviews (in-scope).
	for _, t := range in.Turns {
		resp.KPIs.QuestionsAsked++
		if t.Answered {
			resp.KPIs.QuestionsAnswered++
		}
	}

	if confN > 0 {
		resp.KPIs.AvgConfidence = confSum / float64(confN)
	}
	resp.KPIs.Candidates = len(candidateNames)

	// Verdict mix — always all 5 decisions in locked order, count >= 0.
	resp.VerdictMix = make([]VerdictCount, 0, len(decisionOrder))
	for _, d := range decisionOrder {
		resp.VerdictMix = append(resp.VerdictMix, VerdictCount{Decision: d, Count: verdictCounts[d]})
	}

	resp.Coverage = Coverage{
		ByType:  sortedKeyCounts(byType),
		ByLevel: sortedKeyCounts(byLevel),
	}

	// ── Competency trends (grouped by candidate name) ─────────────────────────
	resp.CompetencyTrends, resp.KPIs.MostImprovedCompetency = buildTrends(interviews, compByIv, in.CandidateName, now)

	// ── Competency profile (latest-per-competency across whole portfolio) ─────
	resp.CompetencyProfile = buildProfile(interviews, compByIv)

	// ── Risks (category × severity) ───────────────────────────────────────────
	resp.Risks = buildRisks(in.Risks)

	// ── Top signals (frequency desc) ──────────────────────────────────────────
	resp.TopSignals = buildSignals(in.Signals)

	// ── Persona × competency averages ─────────────────────────────────────────
	resp.PersonaCompetency = buildPersonaCompetency(in.Recs)

	// ── Within-interview score evolution ──────────────────────────────────────
	resp.ScoreEvolution = buildScoreEvolution(interviews, recByIv, in.Snapshots, in.CandidateName)

	return resp
}

// sortedKeyCounts returns label/count pairs sorted by count desc, then key asc.
func sortedKeyCounts(m map[string]int) []KeyCount {
	out := make([]KeyCount, 0, len(m))
	for k, c := range m {
		out = append(out, KeyCount{Key: k, Count: c})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// buildTrends groups completed interviews by candidate name and runs
// learning.BuildTrends per candidate, then flattens to a portfolio list and
// picks the competency with the largest positive delta as most-improved.
func buildTrends(
	interviews []domain.Interview,
	compByIv map[bson.ObjectID][]domain.CompetencyScore,
	nameByIv map[bson.ObjectID]string,
	now time.Time,
) ([]CompetencyTrend, string) {
	// Group scored interviews by normalized candidate name.
	byName := map[string][]learning.ScoredInterview{}
	for _, iv := range interviews {
		scores := compByIv[iv.ID]
		if len(scores) == 0 {
			continue
		}
		norm := normalizeName(nameByIv[iv.ID])
		byName[norm] = append(byName[norm], learning.ScoredInterview{
			InterviewID: iv.ID,
			At:          iv.CreatedAt,
			Scores:      scores,
		})
	}

	// Stable ordering over candidate names.
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	var out []CompetencyTrend
	mostImproved := ""
	var bestDelta float64
	for _, name := range names {
		// Synthetic candidate ID per name — only used internally by BuildTrends.
		trends := learning.BuildTrends(bson.NewObjectID(), byName[name], now)
		for _, tr := range trends {
			pts := make([]TrendPoint, 0, len(tr.Points))
			for _, p := range tr.Points {
				pts = append(pts, TrendPoint{
					InterviewID: p.InterviewID.Hex(),
					At:          p.At,
					Normal:      p.Normal,
					Confidence:  p.Confidence,
				})
			}
			out = append(out, CompetencyTrend{
				Competency: tr.Competency,
				Direction:  tr.Direction,
				First:      tr.First,
				Latest:     tr.Latest,
				Delta:      tr.Delta,
				Points:     pts,
			})
			// Most-improved requires >= 2 interviews (not a "new" trend).
			if tr.Interviews >= 2 && tr.Delta > bestDelta {
				bestDelta = tr.Delta
				mostImproved = tr.Competency
			}
		}
	}
	return out, mostImproved
}

// buildProfile computes the latest and earliest Normal per competency across
// all completed interviews (oldest-first input assumed). cool/normal/hot/
// confidence reflect the most recent measurement.
func buildProfile(
	interviews []domain.Interview,
	compByIv map[bson.ObjectID][]domain.CompetencyScore,
) []CompetencyProfile {
	type acc struct {
		cool, normal, hot, conf float64
		firstNormal             float64
		seen                    bool
	}
	byComp := map[string]*acc{}
	for _, iv := range interviews { // oldest-first
		for _, cs := range compByIv[iv.ID] {
			a := byComp[cs.Competency]
			if a == nil {
				a = &acc{}
				byComp[cs.Competency] = a
			}
			if !a.seen {
				a.firstNormal = cs.Normal
				a.seen = true
			}
			// Overwrite with the latest (later interview wins).
			a.cool, a.normal, a.hot, a.conf = cs.Cool, cs.Normal, cs.Hot, cs.Confidence
		}
	}

	comps := make([]string, 0, len(byComp))
	for c := range byComp {
		comps = append(comps, c)
	}
	sort.Strings(comps)

	out := make([]CompetencyProfile, 0, len(comps))
	for _, c := range comps {
		a := byComp[c]
		out = append(out, CompetencyProfile{
			Competency:  c,
			Cool:        a.cool,
			Normal:      a.normal,
			Hot:         a.hot,
			Confidence:  a.conf,
			FirstNormal: a.firstNormal,
		})
	}
	return out
}

// buildRisks buckets every risk item by category × severity. Output is ordered
// by category then severity (both using the locked domain order) for stable
// rendering; only non-empty buckets are emitted.
func buildRisks(docs []domain.RiskDoc) []RiskBucket {
	counts := map[string]map[string]int{}
	for _, doc := range docs {
		for _, r := range doc.Risks {
			if counts[r.Category] == nil {
				counts[r.Category] = map[string]int{}
			}
			counts[r.Category][r.Severity]++
		}
	}

	catRank := indexRank(riskCategoryOrder)
	sevRank := indexRank(riskSeverityOrder)

	var out []RiskBucket
	for cat, sevs := range counts {
		for sev, n := range sevs {
			out = append(out, RiskBucket{Category: cat, Severity: sev, Count: n})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := rankOrLast(catRank, out[i].Category), rankOrLast(catRank, out[j].Category)
		if ci != cj {
			return ci < cj
		}
		si, sj := rankOrLast(sevRank, out[i].Severity), rankOrLast(sevRank, out[j].Severity)
		if si != sj {
			return si < sj
		}
		return out[i].Severity < out[j].Severity
	})
	return out
}

// buildSignals counts strongest-signal names across all signal docs, ordered by
// frequency desc then name asc.
func buildSignals(docs []domain.SignalsDoc) []SignalCount {
	counts := map[string]int{}
	for _, doc := range docs {
		for _, sig := range doc.Signals {
			name := strings.TrimSpace(sig.Name)
			if name == "" {
				continue
			}
			counts[name]++
		}
	}
	out := make([]SignalCount, 0, len(counts))
	for n, c := range counts {
		out = append(out, SignalCount{Name: n, Count: c})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// buildPersonaCompetency averages each persona's per-competency scores across
// all recommendations. Personas and competencies are sorted alphabetically.
func buildPersonaCompetency(recs []domain.Recommendation) []PersonaCompetency {
	type stat struct {
		sum float64
		n   int
	}
	// persona -> competency -> stat
	byPersona := map[string]map[string]*stat{}
	for _, rec := range recs {
		for _, pv := range rec.Personas {
			persona := strings.TrimSpace(pv.Persona)
			if persona == "" {
				continue
			}
			if byPersona[persona] == nil {
				byPersona[persona] = map[string]*stat{}
			}
			for _, pc := range pv.PerCompetency {
				comp := strings.TrimSpace(pc.Competency)
				if comp == "" {
					continue
				}
				s := byPersona[persona][comp]
				if s == nil {
					s = &stat{}
					byPersona[persona][comp] = s
				}
				s.sum += pc.Score
				s.n++
			}
		}
	}

	personas := make([]string, 0, len(byPersona))
	for p := range byPersona {
		personas = append(personas, p)
	}
	sort.Strings(personas)

	out := make([]PersonaCompetency, 0, len(personas))
	for _, p := range personas {
		compMap := byPersona[p]
		comps := make([]string, 0, len(compMap))
		for c := range compMap {
			comps = append(comps, c)
		}
		sort.Strings(comps)
		avgs := make([]PersonaCompetencyAvg, 0, len(comps))
		for _, c := range comps {
			s := compMap[c]
			avg := 0.0
			if s.n > 0 {
				avg = s.sum / float64(s.n)
			}
			avgs = append(avgs, PersonaCompetencyAvg{Competency: c, AvgScore: avg})
		}
		out = append(out, PersonaCompetency{Persona: p, Competencies: avgs})
	}
	return out
}

// buildScoreEvolution builds, per completed interview, the mean Normal across
// competencies at each turn from the confidence snapshots.
func buildScoreEvolution(
	interviews []domain.Interview,
	recByIv map[bson.ObjectID]domain.Recommendation,
	snapshots []domain.ConfidenceSnapshot,
	nameByIv map[bson.ObjectID]string,
) []ScoreEvolution {
	// interview -> turn -> (sum, n) of Normal across competencies.
	type stat struct {
		sum float64
		n   int
	}
	byIvTurn := map[bson.ObjectID]map[int]*stat{}
	for _, snap := range snapshots {
		if byIvTurn[snap.InterviewID] == nil {
			byIvTurn[snap.InterviewID] = map[int]*stat{}
		}
		s := byIvTurn[snap.InterviewID][snap.Turn]
		if s == nil {
			s = &stat{}
			byIvTurn[snap.InterviewID][snap.Turn] = s
		}
		s.sum += snap.Normal
		s.n++
	}

	var out []ScoreEvolution
	for _, iv := range interviews { // oldest-first
		if _, completed := recByIv[iv.ID]; !completed {
			continue
		}
		turnMap := byIvTurn[iv.ID]
		if len(turnMap) == 0 {
			continue
		}
		turns := make([]int, 0, len(turnMap))
		for t := range turnMap {
			turns = append(turns, t)
		}
		sort.Ints(turns)
		series := make([]ScoreEvoPoint, 0, len(turns))
		for _, t := range turns {
			s := turnMap[t]
			avg := 0.0
			if s.n > 0 {
				avg = s.sum / float64(s.n)
			}
			series = append(series, ScoreEvoPoint{Turn: t, AvgNormal: avg})
		}
		out = append(out, ScoreEvolution{
			InterviewID:   iv.ID.Hex(),
			CandidateName: nameByIv[iv.ID],
			Type:          iv.Type,
			Series:        series,
		})
	}
	return out
}

// indexRank maps each value to its position for stable ordering.
func indexRank(order []string) map[string]int {
	m := make(map[string]int, len(order))
	for i, v := range order {
		m[v] = i
	}
	return m
}

// rankOrLast returns the rank of v, or a large value if v is unknown so that
// unexpected labels sort last rather than panicking.
func rankOrLast(rank map[string]int, v string) int {
	if r, ok := rank[v]; ok {
		return r
	}
	return len(rank) + 1
}
