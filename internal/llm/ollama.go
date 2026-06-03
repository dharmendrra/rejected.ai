package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// OllamaCaller implements Caller and Streamer against a local Ollama server
// (/api/generate). It mirrors the patterns in agentic-ai/agents/llm.go and
// Omni-RAG/pinecone-rag/retrieval/streamer.go.
type OllamaCaller struct {
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
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
	return map[string]any{
		"num_predict": o.MaxTokens,
		"temperature": o.Temperature,
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

// Stream performs a streaming generation, invoking onToken for each chunk.
func (o *OllamaCaller) Stream(ctx context.Context, system, user string, onToken func(string) error) error {
	log.Printf("[OLLAMA] stream model=%s", o.Model)
	reqBody := ollamaGenerateRequest{
		Model:   o.Model,
		Prompt:  user,
		System:  system,
		Stream:  true,
		Options: o.options(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama stream status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk ollamaGenerateChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decode ollama stream chunk: %w", err)
		}
		if chunk.Response != "" {
			if err := onToken(chunk.Response); err != nil {
				return err
			}
		}
		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}

// OllamaEmbedder implements Embedder against /api/embed.
type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (o *OllamaEmbedder) ModelName() string { return o.Model }

type ollamaEmbedResponse struct {
	Embedding  []float32   `json:"embedding"`
	Embeddings [][]float32 `json:"embeddings"`
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("cannot embed empty text")
	}

	body, err := json.Marshal(map[string]any{
		"model": o.Model,
		"input": []string{text},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama embed status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if len(decoded.Embeddings) > 0 && len(decoded.Embeddings[0]) > 0 {
		return decoded.Embeddings[0], nil
	}
	if len(decoded.Embedding) > 0 {
		return decoded.Embedding, nil
	}
	return nil, errors.New("ollama embed returned no vector")
}
