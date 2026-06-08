package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// OllamaCaller implements Caller against a local Ollama server (/api/generate).
// It mirrors the patterns in agentic-ai/agents/llm.go.
type OllamaCaller struct {
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
	NumCtx      int
	Client      *http.Client
}

func (o *OllamaCaller) ModelName() string { return o.Model }

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (o *OllamaCaller) options() map[string]any {
	numCtx := o.NumCtx
	if numCtx <= 0 {
		numCtx = 16384
	}
	return map[string]any{
		"num_predict": o.MaxTokens,
		"temperature": o.Temperature,
		"num_ctx":     numCtx,
	}
}

// Call performs a non-streaming generation.
func (o *OllamaCaller) Call(ctx context.Context, system, user string) (string, error) {
	log.Printf("[OLLAMA] generate model=%s max_tokens=%d", o.Model, o.MaxTokens)
	reqBody := ollamaGenerateRequest{
		Model:   o.Model,
		Prompt:  user,
		System:  system,
		Stream:  false,
		Options: o.options(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("ollama generate status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var chunk ollamaGenerateChunk
	if err := json.NewDecoder(resp.Body).Decode(&chunk); err != nil {
		return "", fmt.Errorf("decode ollama generate response: %w", err)
	}
	return chunk.Response, nil
}
