package api

import (
	"context"
	"net/http"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/learning"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// trendsView is the read model returned by the candidate-trends endpoints: the
// per-competency trajectories plus an at-a-glance pattern summary.
type trendsView struct {
	CandidateID bson.ObjectID            `json:"candidate_id"`
	Trends      []domain.HistoricalTrend `json:"trends"`
	Improving   []string                 `json:"improving"`
	Declining   []string                 `json:"declining"`
	Stable      []string                 `json:"stable"`
}

func newTrendsView(candidateID bson.ObjectID, trends []domain.HistoricalTrend) trendsView {
	up, down, stable := learning.Summarize(trends)
	return trendsView{
		CandidateID: candidateID,
		Trends:      trends,
		Improving:   up,
		Declining:   down,
		Stable:      stable,
	}
}

// handleComputeTrends (re)computes a candidate's cross-interview trends from
// stored competency scores and persists them. Deterministic, no LLM calls.
func (s *Server) handleComputeTrends(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid candidate id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()

	trends, err := s.Learning.ComputeForCandidate(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newTrendsView(id, trends))
}

// handleGetTrends returns a candidate's previously computed trends.
func (s *Server) handleGetTrends(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid candidate id")
		return
	}
	trends, err := s.Learning.Load(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(trends) == 0 {
		writeError(w, http.StatusNotFound, "no trends computed yet; POST to this endpoint to compute")
		return
	}
	writeJSON(w, http.StatusOK, newTrendsView(id, trends))
}
