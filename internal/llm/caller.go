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

// Streamer produces a streaming response, invoking onToken for each chunk.
type Streamer interface {
	Stream(ctx context.Context, system, user string, onToken func(string) error) error
	ModelName() string
}

// Embedder turns text into a vector embedding.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	ModelName() string
}

// Provider bundles the three capabilities so dependents can take one dependency.
// Embedding always uses Ollama (nomic-embed-text); generation/streaming follow
// the configured backend.
type Provider struct {
	Caller   Caller
	Streamer Streamer
	Embedder Embedder
}

// New builds a Provider from config. Generation defaults to the configured
// backend ("ollama" or "anthropic"); embeddings always use local Ollama.
func New(cfg *config.Config) (*Provider, error) {
	httpClient := &http.Client{Timeout: 15 * time.Minute}

	embedder := &OllamaEmbedder{
		BaseURL: cfg.OllamaHost,
		Model:   cfg.OllamaEmbedModel,
		Client:  httpClient,
	}

	switch cfg.LLMBackend {
	case "ollama":
		o := &OllamaCaller{
			BaseURL:     cfg.OllamaHost,
			Model:       cfg.OllamaModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
			NumCtx:      cfg.OllamaNumCtx,
			Client:      httpClient,
		}
		return &Provider{Caller: o, Streamer: o, Embedder: embedder}, nil

	case "anthropic":
		a := &AnthropicCaller{
			APIKey:      cfg.AnthropicAPIKey,
			Model:       cfg.AnthropicModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
			Client:      httpClient,
		}
		// Anthropic streaming is added in a later phase; fall back to the
		// Ollama streamer so live endpoints still function locally.
		ollamaStreamer := &OllamaCaller{
			BaseURL:     cfg.OllamaHost,
			Model:       cfg.OllamaModel,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
			NumCtx:      cfg.OllamaNumCtx,
			Client:      httpClient,
		}
		return &Provider{Caller: a, Streamer: ollamaStreamer, Embedder: embedder}, nil

	default:
		return nil, fmt.Errorf("unknown LLM backend %q", cfg.LLMBackend)
	}
}
