// Package api wires the HTTP layer using the stdlib net/http ServeMux
// (Go 1.22+ method+path patterns). It holds shared dependencies and the engine
// services, and registers routes. New handlers are added per phase.
package api

import (
	"net/http"

	"github.com/dharmendra/rejected.ai/internal/assumptions"
	"github.com/dharmendra/rejected.ai/internal/capability"
	"github.com/dharmendra/rejected.ai/internal/confidence"
	"github.com/dharmendra/rejected.ai/internal/config"
	"github.com/dharmendra/rejected.ai/internal/documents"
	"github.com/dharmendra/rejected.ai/internal/evaluators"
	"github.com/dharmendra/rejected.ai/internal/evidence"
	"github.com/dharmendra/rejected.ai/internal/interview"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/media"
	"github.com/dharmendra/rejected.ai/internal/recommendation"
	"github.com/dharmendra/rejected.ai/internal/report"
	"github.com/dharmendra/rejected.ai/internal/risk"
	"github.com/dharmendra/rejected.ai/internal/signals"
	"github.com/dharmendra/rejected.ai/internal/store"
)

// Server bundles dependencies shared across handlers.
type Server struct {
	Cfg   *config.Config
	Store *store.Store
	LLM   *llm.Provider

	Documents *documents.Service
	Interview *interview.Service
	Report    *report.Service
	Media     *media.Service
}

// NewServer constructs a Server and the engine services it exposes.
func NewServer(cfg *config.Config, st *store.Store, provider *llm.Provider) *Server {
	ev := evidence.NewService(provider, st)
	conf := confidence.NewService(provider, st, ev)
	cap := capability.NewService(provider)
	asm := assumptions.NewService(provider)

	eval := evaluators.NewService(provider)
	sig := signals.NewService(provider)
	rk := risk.NewService(provider)
	rec := recommendation.NewService(provider)

	// Keep the interface nil (not a typed-nil) when whisper isn't configured,
	// so media.Service can detect the absence correctly.
	var transcriber media.Transcriber
	if w := media.NewWhisperCpp(cfg.WhisperBin, cfg.WhisperModel); w != nil {
		transcriber = w
	}

	return &Server{
		Cfg:       cfg,
		Store:     st,
		LLM:       provider,
		Documents: documents.NewService(provider, st),
		Interview: interview.NewService(provider, st, cap, ev, conf, asm),
		Report:    report.NewService(st, ev, conf, eval, sig, rk, rec),
		Media:     media.NewService(st, transcriber),
	}
}

// Routes builds the HTTP handler with all registered routes.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Phase 1 — ingestion.
	mux.HandleFunc("POST /api/job-descriptions", s.handleIngestJD)
	mux.HandleFunc("POST /api/resumes", s.handleIngestResume)

	// Phase 3 — interview sessions.
	mux.HandleFunc("POST /api/interviews", s.handleCreateInterview)
	mux.HandleFunc("POST /api/interviews/{id}/answer", s.handleSubmitAnswer)
	mux.HandleFunc("GET /api/interviews/{id}", s.handleGetInterview)

	// Phase 7 — final report (generate / fetch cached).
	mux.HandleFunc("POST /api/interviews/{id}/report", s.handleGenerateReport)
	mux.HandleFunc("GET /api/interviews/{id}/report", s.handleGetReport)

	// Phase 9 — audio: supply a transcript, or upload audio (if whisper configured).
	mux.HandleFunc("POST /api/interviews/{id}/transcript", s.handleIngestTranscript)
	mux.HandleFunc("POST /api/interviews/{id}/audio", s.handleIngestAudio)

	return withLogging(withCORS(mux))
}
