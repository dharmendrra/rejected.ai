// Package llm provides pluggable LLM backends (Ollama and Anthropic) behind
// small interfaces, so generation, streaming, and embedding can be swapped via
// config without touching call sites. This mirrors the LLMCaller / Streamer /
// Embedder pattern used in the user's agentic-ai and Omni-RAG projects.
package llm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dharmendra/rejected.ai/internal/config"
)

// Caller produces a single complete response from a system + user prompt.
type Caller interface {
	Call(ctx context.Context, system, user string) (string, error)
	ModelName() string
}

// Provider bundles the LLM capabilities dependents need behind one dependency.
type Provider struct {
	Caller Caller
}

// New builds a Provider from config. Generation uses the configured backend
// ("ollama" or "anthropic"). When LLM_LOG_LEVEL is "info" or "debug", every
// call is wrapped with audit logging to logs/llm_calls.log.
func New(cfg *config.Config) (*Provider, error) {
	httpClient := &http.Client{Timeout: 15 * time.Minute}

	var caller Caller
	switch cfg.LLMBackend {
	case "ollama":
		caller = &OllamaCaller{
			BaseURL:     cfg.OllamaHost,
			Model:       cfg.OllamaModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: *cfg.Temperature,
			NumCtx:      cfg.OllamaNumCtx,
			Client:      httpClient,
		}
	case "anthropic":
		caller = &AnthropicCaller{
			APIKey:      cfg.AnthropicAPIKey,
			Model:       cfg.AnthropicModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: *cfg.Temperature,
			Client:      httpClient,
		}
	default:
		return nil, fmt.Errorf("unknown LLM backend %q", cfg.LLMBackend)
	}

	if lvl := cfg.LLMLogLevel; lvl != "" && lvl != "off" {
		logger, err := newAuditLogger(lvl)
		if err != nil {
			return nil, err
		}
		log.Printf("[LLM] audit logging enabled (level=%s) -> %s", lvl, auditLogPath)
		caller = &loggingCaller{inner: caller, logger: logger}
	}

	return &Provider{Caller: caller}, nil
}
