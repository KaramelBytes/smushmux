package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient is a minimal HTTP client for a local Ollama runtime.
// It implements a Generate method compatible with the OpenRouter client surface.
type OllamaClient struct {
	httpClient       *http.Client
	host             string
	retryMaxAttempts int
	retryBaseDelay   time.Duration
	retryMaxDelay    time.Duration
}

// NewOllamaClient creates a new client targeting the given host (e.g., http://127.0.0.1:11434).
func NewOllamaClient(host string, httpTimeout time.Duration, retryMax int, baseDelay, maxDelay time.Duration) *OllamaClient {
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	if httpTimeout <= 0 {
		httpTimeout = 60 * time.Second
	}
	if retryMax <= 0 {
		retryMax = 2
	}
	if baseDelay <= 0 {
		baseDelay = 200 * time.Millisecond
	}
	if maxDelay <= 0 {
		maxDelay = 1 * time.Second
	}
	return &OllamaClient{
		httpClient:       &http.Client{Timeout: httpTimeout},
		host:             host,
		retryMaxAttempts: retryMax,
		retryBaseDelay:   baseDelay,
		retryMaxDelay:    maxDelay,
	}
}

// Structures aligned with Ollama /api/chat (non-streaming)
type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
}
type ollamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	// Other fields omitted
	Done bool `json:"done"`
}

// Generate sends a chat request to Ollama and maps the response to GenerateResponse.
func (c *OllamaClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if req.Model == "" {
		return nil, errors.New("model cannot be empty")
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("messages cannot be empty")
	}

	// Convert all messages to Ollama format
	messages := make([]ollamaChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = ollamaChatMessage{ //nolint:gosimple
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	oreq := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
		Options:  map[string]any{},
	}
	if req.Temperature > 0 {
		oreq.Options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		oreq.Options["num_predict"] = req.MaxTokens
	}

	payload, err := json.Marshal(oreq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.host + "/api/chat"
	maxAttempts := c.retryMaxAttempts
	backoff := c.retryBaseDelay
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			// Retry transient network errors
			if isRetryableNetErr(err) && attempt < maxAttempts {
				time.Sleep(withJitter(backoff))
				backoff *= 2
				continue
			}
			return nil, &UnreachableError{Host: c.host, Err: err}
		}
		var out GenerateResponse
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
				var raw map[string]any
				_ = json.Unmarshal(body, &raw)
				apiErr := &APIError{StatusCode: resp.StatusCode, Raw: raw}
				if msg, ok := raw["error"].(string); ok {
					apiErr.Message = msg
				}
				if msg, ok := raw["message"].(string); ok && apiErr.Message == "" {
					apiErr.Message = msg
				}
				// classifying broadly
				if resp.StatusCode == http.StatusNotFound {
					// Likely missing model
					lastErr = &ModelNotFoundError{APIError: apiErr}
					return
				}
				if resp.StatusCode >= 500 {
					lastErr = &ServerError{APIError: apiErr}
					return
				}
				if resp.StatusCode == 400 {
					lastErr = &BadRequestError{APIError: apiErr}
					return
				}
				lastErr = apiErr
				return
			}
			var oresp ollamaChatResponse
			if err := json.NewDecoder(resp.Body).Decode(&oresp); err != nil {
				lastErr = fmt.Errorf("decode response: %w", err)
				return
			}
			out.Choices = []Choice{{Message: Message{Role: "assistant", Content: oresp.Message.Content}}}
			// Simulated correlation id
			out.RequestID = fmt.Sprintf("ollama_%d", time.Now().UnixNano())
			lastErr = nil
		}()
		if lastErr == nil {
			return &out, nil
		}
		if attempt < maxAttempts {
			time.Sleep(withJitter(backoff))
			backoff *= 2
			continue
		}
		break
	}
	return nil, lastErr
}

// network retry helper is shared from client.go (same package)

// GenerateStream streams partial deltas from Ollama when supported.
func (c *OllamaClient) GenerateStream(ctx context.Context, req GenerateRequest, onDelta func(string)) error {
	if req.Model == "" {
		return errors.New("model cannot be empty")
	}
	if len(req.Messages) == 0 {
		return errors.New("messages cannot be empty")
	}

	// Convert all messages to Ollama format
	messages := make([]ollamaChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = ollamaChatMessage{ //nolint:gosimple
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	oreq := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
		Options:  map[string]any{},
	}
	if req.Temperature > 0 {
		oreq.Options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		oreq.Options["num_predict"] = req.MaxTokens
	}
	payload, err := json.Marshal(oreq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.host + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &UnreachableError{Host: c.host, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		var raw map[string]any
		_ = json.Unmarshal(body, &raw)
		apiErr := &APIError{StatusCode: resp.StatusCode, Raw: raw}
		if msg, ok := raw["error"].(string); ok {
			apiErr.Message = msg
		}
		if msg, ok := raw["message"].(string); ok && apiErr.Message == "" {
			apiErr.Message = msg
		}
		if resp.StatusCode == http.StatusNotFound {
			return &ModelNotFoundError{APIError: apiErr}
		}
		if resp.StatusCode >= 500 {
			return &ServerError{APIError: apiErr}
		}
		if resp.StatusCode == 400 {
			return &BadRequestError{APIError: apiErr}
		}
		return apiErr
	}

	dec := json.NewDecoder(resp.Body)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var oresp ollamaChatResponse
		if err := dec.Decode(&oresp); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("decode stream: %w", err)
		}
		if msg := oresp.Message.Content; msg != "" {
			onDelta(msg)
		}
		if oresp.Done {
			break
		}
	}
	return nil
}
