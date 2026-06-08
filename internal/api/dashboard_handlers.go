package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dharmendra/rejected.ai/internal/dashboard"
	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// handleDashboard returns the aggregated portfolio view for the Progress
// Dashboard. It does the batched Mongo reads (one $in query per related
// collection, mirroring handleListInterviews to avoid N+1), then hands the
// loaded documents to the pure dashboard.Aggregate.
//
// Optional query params:
//   - candidate=<name>  scope to one candidate name (default: all)
//   - from=<RFC3339>    only interviews created on/after this time
//   - to=<RFC3339>      only interviews created on/before this time
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// ── Parse scope filters ───────────────────────────────────────────────────
	candidateFilter := strings.TrimSpace(r.URL.Query().Get("candidate"))
	scope := dashboard.Scope{Candidate: "all"}
	if candidateFilter != "" && !strings.EqualFold(candidateFilter, "all") {
		scope.Candidate = candidateFilter
	}

	var fromPtr, toPtr *time.Time
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'from' (want RFC3339): "+err.Error())
			return
		}
		fromPtr = &t
		scope.From = &t
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'to' (want RFC3339): "+err.Error())
			return
		}
		toPtr = &t
		scope.To = &t
	}

	// ── Load all interviews (date-range filtered), sorted by created_at ───────
	ivFilter := bson.D{}
	if fromPtr != nil || toPtr != nil {
		rng := bson.D{}
		if fromPtr != nil {
			rng = append(rng, bson.E{Key: "$gte", Value: *fromPtr})
		}
		if toPtr != nil {
			rng = append(rng, bson.E{Key: "$lte", Value: *toPtr})
		}
		ivFilter = bson.D{{Key: "created_at", Value: rng}}
	}

	cur, err := s.Store.Coll(store.CollInterviews).Find(ctx, ivFilter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		log.Printf("[API] dashboard: list interviews: %v", err)
		writeError(w, http.StatusInternalServerError, "list interviews: "+err.Error())
		return
	}
	var interviews []domain.Interview
	if err := cur.All(ctx, &interviews); err != nil {
		writeError(w, http.StatusInternalServerError, "decode interviews: "+err.Error())
		return
	}

	// ── Resolve candidate names + job titles (batched), then candidate scope ──
	candidateIDs := make([]bson.ObjectID, 0, len(interviews))
	jdIDs := make([]bson.ObjectID, 0, len(interviews))
	for _, iv := range interviews {
		candidateIDs = append(candidateIDs, iv.CandidateProfileID)
		jdIDs = append(jdIDs, iv.JobDescriptionID)
	}

	nameByProfile := map[bson.ObjectID]string{}
	if c, err := s.Store.Coll(store.CollCandidateProfile).Find(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: candidateIDs}}}}); err == nil {
		var cps []domain.CandidateProfile
		_ = c.All(ctx, &cps)
		for _, cp := range cps {
			nameByProfile[cp.ID] = cp.Name
		}
	}
	titleByJD := map[bson.ObjectID]string{}
	if c, err := s.Store.Coll(store.CollJobDescriptions).Find(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: jdIDs}}}}); err == nil {
		var jds []domain.JobDescription
		_ = c.All(ctx, &jds)
		for _, jd := range jds {
			titleByJD[jd.ID] = jd.Title
		}
	}

	// Build interview-ID -> name / title lookups; apply candidate-name scope.
	nameByIv := map[bson.ObjectID]string{}
	titleByIv := map[bson.ObjectID]string{}
	scoped := make([]domain.Interview, 0, len(interviews))
	wantName := strings.ToLower(strings.TrimSpace(candidateFilter))
	filterCandidate := wantName != "" && wantName != "all"
	for _, iv := range interviews {
		name := nameByProfile[iv.CandidateProfileID]
		if filterCandidate && strings.ToLower(strings.TrimSpace(name)) != wantName {
			continue
		}
		nameByIv[iv.ID] = name
		titleByIv[iv.ID] = titleByJD[iv.JobDescriptionID]
		scoped = append(scoped, iv)
	}
	interviews = scoped

	// Interview IDs for the remaining batched reads.
	interviewIDs := make([]bson.ObjectID, 0, len(interviews))
	for _, iv := range interviews {
		interviewIDs = append(interviewIDs, iv.ID)
	}

	// ── Batched reads for every chart dataset ─────────────────────────────────
	in := dashboard.Input{
		Interviews:    interviews,
		Recs:          findIn[domain.Recommendation](ctx, s, store.CollRecommendations, interviewIDs),
		Competency:    findIn[domain.CompetencyScore](ctx, s, store.CollCompetencyScores, interviewIDs),
		Risks:         findIn[domain.RiskDoc](ctx, s, store.CollRiskAreas, interviewIDs),
		Signals:       findIn[domain.SignalsDoc](ctx, s, store.CollSignals, interviewIDs),
		Snapshots:     findIn[domain.ConfidenceSnapshot](ctx, s, store.CollConfidenceScores, interviewIDs),
		Turns:         findIn[domain.Turn](ctx, s, store.CollQuestions, interviewIDs),
		CandidateName: nameByIv,
		JobTitle:      titleByIv,
	}

	resp := dashboard.Aggregate(in, scope, time.Now().UTC())
	log.Printf("[API] dashboard: scope=%q interviews=%d completed=%d", scope.Candidate, resp.KPIs.TotalInterviews, resp.KPIs.CompletedReports)
	writeJSON(w, http.StatusOK, resp)
}

// findIn loads all docs from a collection whose interview_id is in ids. Returns
// an empty slice (never nil-dependent behavior) on error so the aggregator can
// run on whatever loaded. With no ids it skips the query entirely.
func findIn[T any](ctx context.Context, s *Server, coll string, ids []bson.ObjectID) []T {
	if len(ids) == 0 {
		return nil
	}
	cur, err := s.Store.Coll(coll).Find(ctx,
		bson.D{{Key: "interview_id", Value: bson.D{{Key: "$in", Value: ids}}}})
	if err != nil {
		log.Printf("[API] dashboard: load %s: %v", coll, err)
		return nil
	}
	var out []T
	_ = cur.All(ctx, &out)
	return out
}
