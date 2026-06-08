// Package config loads runtime configuration from a JSON file.
//
// It follows the same convention used elsewhere in the user's projects
// (agentic-ai/agents/config.go): a single config.json read at startup.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all runtime settings for the rejected.ai server.
type Config struct {
	ServerAddr string `json:"SERVER_ADDR"`

	MongoURI string `json:"MONGO_URI"`
	MongoDB  string `json:"MONGO_DB"`

	// LLMBackend selects the default generation/evaluation backend: "ollama" or "anthropic".
	LLMBackend string `json:"LLM_BACKEND"`

	OllamaHost       string `json:"OLLAMA_HOST"`
	OllamaModel      string `json:"OLLAMA_MODEL"`
	OllamaEmbedModel string `json:"OLLAMA_EMBED_MODEL"`

	AnthropicAPIKey string `json:"ANTHROPIC_API_KEY"`
	AnthropicModel  string `json:"ANTHROPIC_MODEL"`

	MaxTokens   int     `json:"MAX_TOKENS"`
	Temperature float64 `json:"TEMPERATURE"`
	OllamaNumCtx int     `json:"OLLAMA_NUM_CTX"`

	// Audio (Phase 9). When both are set, audio uploads are transcribed via a
	// whisper.cpp CLI; otherwise callers supply transcripts directly.
	WhisperBin   string `json:"WHISPER_BIN"`
	WhisperModel string `json:"WHISPER_MODEL"`

	// Video (Phase 10). When set, video uploads are inspected by an external
	// detector CLI that emits FrameMetrics JSON; otherwise callers supply frame
	// metrics directly. The model path is optional and detector-specific.
	VideoDetectorBin   string `json:"VIDEO_DETECTOR_BIN"`
	VideoDetectorModel string `json:"VIDEO_DETECTOR_MODEL"`
}

// Load reads and parses the config file at path, then applies defaults and validation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.ServerAddr == "" {
		c.ServerAddr = ":8080"
	}
	if c.MongoURI == "" {
		c.MongoURI = "mongodb://localhost:27017"
	}
	if c.MongoDB == "" {
		c.MongoDB = "rejected_ai"
	}
	if c.LLMBackend == "" {
		c.LLMBackend = "ollama"
	}
	if c.OllamaHost == "" {
		c.OllamaHost = "http://localhost:11434"
	}
	if c.OllamaModel == "" {
		c.OllamaModel = "gemma4:e4b"
	}
	if c.OllamaEmbedModel == "" {
		c.OllamaEmbedModel = "nomic-embed-text"
	}
	if c.AnthropicModel == "" {
		c.AnthropicModel = "claude-sonnet-4-6"
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = 4096
	}
	if c.Temperature == 0 {
		c.Temperature = 0.4
	}
	if c.OllamaNumCtx == 0 {
		c.OllamaNumCtx = 16384
	}
}

func (c *Config) validate() error {
	switch c.LLMBackend {
	case "ollama":
		// Ollama needs only a reachable host; checked at call time.
	case "anthropic":
		if c.AnthropicAPIKey == "" {
			return fmt.Errorf("LLM_BACKEND=anthropic requires ANTHROPIC_API_KEY")
		}
	default:
		return fmt.Errorf("unknown LLM_BACKEND %q (want \"ollama\" or \"anthropic\")", c.LLMBackend)
	}
	return nil
}
