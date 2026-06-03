package api

import (
	"context"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// handleGenerateReport computes the final assessment (evaluator panel, signals,
// risk, recommendation). This makes several LLM calls and can be slow.
func (s *Server) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	rep, err := s.Report.Generate(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// handleGetReport returns a previously generated report from stored documents.
func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	rep, err := s.Report.Load(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if rep.Recommendation == nil {
		writeError(w, http.StatusNotFound, "report not generated yet; POST to this endpoint to generate")
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
