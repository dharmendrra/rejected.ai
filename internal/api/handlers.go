package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleHealthz reports liveness and dependency health (Mongo + configured LLM).
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp := map[string]any{
		"status":      "ok",
		"llm_backend": s.Cfg.LLMBackend,
		"llm_model":   s.LLM.Caller.ModelName(),
		"mongo":       "ok",
	}

	if err := s.Store.Ping(ctx); err != nil {
		resp["status"] = "degraded"
		resp["mongo"] = "error: " + err.Error()
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
