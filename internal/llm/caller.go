// Package llm provides pluggable LLM backends (Ollama and Anthropic) behind
// small interfaces, so generation, streaming, and embedding can be swapped via
// config without touching call sites. This mirrors the LLMCaller / Streamer /
// Embedder pattern used in the user's agentic-ai and Omni-RAG projects.
package llm

import (
	"context"
	"fmt"
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
// ("ollama" or "anthropic").
func New(cfg *config.Config) (*Provider, error) {
	httpClient := &http.Client{Timeout: 15 * time.Minute}

	switch cfg.LLMBackend {
	case "ollama":
		o := &OllamaCaller{
			BaseURL:     cfg.OllamaHost,
			Model:       cfg.OllamaModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: *cfg.Temperature,
			NumCtx:      cfg.OllamaNumCtx,
			Client:      httpClient,
		}
		return &Provider{Caller: o}, nil

	case "anthropic":
		a := &AnthropicCaller{
			APIKey:      cfg.AnthropicAPIKey,
			Model:       cfg.AnthropicModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: *cfg.Temperature,
			Client:      httpClient,
		}
		return &Provider{Caller: a}, nil

	default:
		return nil, fmt.Errorf("unknown LLM backend %q", cfg.LLMBackend)
	}
}
