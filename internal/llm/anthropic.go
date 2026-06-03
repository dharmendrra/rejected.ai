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

const anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// AnthropicCaller implements Caller against the Anthropic Messages API.
type AnthropicCaller struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Client      *http.Client
}

func (a *AnthropicCaller) ModelName() string { return a.Model }

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *AnthropicCaller) Call(ctx context.Context, system, user string) (string, error) {
	log.Printf("[ANTHROPIC] generate model=%s max_tokens=%d", a.Model, a.MaxTokens)
	reqBody := anthropicRequest{
		Model:       a.Model,
		MaxTokens:   a.MaxTokens,
		Temperature: a.Temperature,
		System:      system,
		Messages:    []anthropicMessage{{Role: "user", Content: user}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var decoded anthropicResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decode anthropic response (status %d): %w", resp.StatusCode, err)
	}
	if decoded.Error != nil {
		return "", fmt.Errorf("anthropic error: %s: %s", decoded.Error.Type, decoded.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var sb strings.Builder
	for _, block := range decoded.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String(), nil
}
