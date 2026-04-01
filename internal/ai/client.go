package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	httpClient       *http.Client
	apiKey           string
	baseURL          string
	retryMaxAttempts int
	retryBaseDelay   time.Duration
	retryMaxDelay    time.Duration
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Choice struct {
	Message Message `json:"message"`
}

type GenerateResponse struct {
	ID        string   `json:"id"`
	Choices   []Choice `json:"choices"`
	Usage     Usage    `json:"usage"`
	RequestID string   `json:"-"`
}

// Embeddings
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// APIError represents a structured API error response.
type APIError struct {
	StatusCode int            `json:"-"`
	Code       string         `json:"code,omitempty"`
	Message    string         `json:"message,omitempty"`
	Raw        map[string]any `json:"-"`
	RequestID  string         `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		if e.Code != "" {
			if e.RequestID != "" {
				return fmt.Sprintf("api error: status=%d code=%s request_id=%s message=%s", e.StatusCode, e.Code, e.RequestID, e.Message)
			}
			return fmt.Sprintf("api error: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
		}
		if e.RequestID != "" {
			return fmt.Sprintf("api error: status=%d request_id=%s message=%s", e.StatusCode, e.RequestID, e.Message)
		}
		return fmt.Sprintf("api error: status=%d message=%s", e.StatusCode, e.Message)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("api error: status=%d request_id=%s", e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("api error: status=%d", e.StatusCode)
}

// NewOpenRouterClient returns a client with default timeouts and retry strategy.
func NewOpenRouterClient(apiKey string) *Client {
	return NewClient(apiKey, 60*time.Second, 3, 500*time.Millisecond, 4*time.Second)
}

// NewClient allows customizing HTTP timeout and retry/backoff behavior.
func NewClient(apiKey string, httpTimeout time.Duration, retryMax int, baseDelay, maxDelay time.Duration) *Client {
	if httpTimeout <= 0 {
		httpTimeout = 60 * time.Second
	}
	if retryMax <= 0 {
		retryMax = 3
	}
	if baseDelay <= 0 {
		baseDelay = 500 * time.Millisecond
	}
	if maxDelay <= 0 {
		maxDelay = 4 * time.Second
	}
	return &Client{
		httpClient:       &http.Client{Timeout: httpTimeout},
		apiKey:           apiKey,
		baseURL:          "https://openrouter.ai/api/v1",
		retryMaxAttempts: retryMax,
		retryBaseDelay:   baseDelay,
		retryMaxDelay:    maxDelay,
	}
}

// NewClientWithBaseURL allows injecting a custom base URL (used in tests).
func NewClientWithBaseURL(apiKey string, httpTimeout time.Duration, retryMax int, baseDelay, maxDelay time.Duration, baseURL string) *Client {
	c := NewClient(apiKey, httpTimeout, retryMax, baseDelay, maxDelay)
	if baseURL != "" {
		c.baseURL = baseURL
	}
	return c
}

func (c *Client) ValidateModel(model string) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}
	return nil
}

func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if c.apiKey == "" {
		return nil, errors.New("OPENROUTER_API_KEY is missing")
	}
	if err := c.ValidateModel(req.Model); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	endpoint := c.baseURL + "/chat/completions"
	// retry settings from client config
	maxAttempts := c.retryMaxAttempts
	backoff := c.retryBaseDelay
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	// No need to seed global rand per Go 1.20

	var lastErr error
	var out GenerateResponse
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Respect context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("HTTP-Referer", "https://github.com/KaramelBytes/smushmux")
		httpReq.Header.Set("X-Title", "SmushMux CLI")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			// network errors: potentially retryable
			if isRetryableNetErr(err) && attempt < maxAttempts {
				lastErr = err
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("http request: %w", err)
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				// Try to decode structured error
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
				var raw map[string]any
				_ = json.Unmarshal(body, &raw)
				apiErr := &APIError{StatusCode: resp.StatusCode, Raw: raw}
				// Capture request id if provider returns one
				apiErr.RequestID = extractRequestID(resp)
				if v, ok := raw["error"].(map[string]any); ok {
					if msg, ok := v["message"].(string); ok {
						apiErr.Message = msg
					}
					if code, ok := v["code"].(string); ok {
						apiErr.Code = code
					}
				} else {
					if msg, ok := raw["message"].(string); ok {
						apiErr.Message = msg
					}
					if code, ok := raw["code"].(string); ok {
						apiErr.Code = code
					}
				}
				// Retry classification
				if (resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 599)) && attempt < maxAttempts {
					// Respect Retry-After header if present (seconds or HTTP date).
					if ra := resp.Header.Get("Retry-After"); ra != "" {
						if secs, err := parseRetryAfterSeconds(ra); err == nil && secs > 0 {
							lastErr = &RateLimitError{APIError: apiErr, RetryAfter: time.Duration(secs) * time.Second}
							time.Sleep(time.Duration(secs) * time.Second)
							return
						}
					}
					lastErr = apiErr
					// exponential backoff with cap
					sleep := withJitter(backoff)
					if c.retryMaxDelay > 0 && sleep > c.retryMaxDelay {
						sleep = c.retryMaxDelay
					}
					time.Sleep(sleep)
					backoff *= 2
					return
				}
				// Non-retryable: return a classified error for better UX
				lastErr = classifyAPIError(apiErr, resp)
				// Non-retryable, exit loop
				return
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				lastErr = fmt.Errorf("decode response: %w", err)
				return
			}
			// capture request id for observability
			out.RequestID = extractRequestID(resp)
			// Success
			lastErr = nil
		}()
		// Success: return parsed response
		if lastErr == nil {
			return &out, nil
		}
		if lastErr != nil && attempt < maxAttempts {
			// retry
			continue
		}
		break
	}
	return nil, lastErr
}

// Embed generates embeddings for the given inputs using the provider's embeddings endpoint.
// Returns one vector per input string.
func (c *Client) Embed(ctx context.Context, model string, inputs []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, errors.New("OPENROUTER_API_KEY is missing")
	}
	if model == "" {
		return nil, errors.New("embedding model cannot be empty")
	}
	if len(inputs) == 0 {
		return nil, errors.New("inputs cannot be empty")
	}
	payload, err := json.Marshal(EmbeddingRequest{Model: model, Input: inputs})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	endpoint := c.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/KaramelBytes/smushmux")
	httpReq.Header.Set("X-Title", "SmushMux CLI")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		var raw map[string]any
		_ = json.Unmarshal(body, &raw)
		apiErr := &APIError{StatusCode: resp.StatusCode, Raw: raw}
		if v, ok := raw["error"].(map[string]any); ok {
			if msg, ok := v["message"].(string); ok {
				apiErr.Message = msg
			}
			if code, ok := v["code"].(string); ok {
				apiErr.Code = code
			}
		}
		return nil, classifyAPIError(apiErr, resp)
	}
	var out EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	vectors := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		vec := make([]float32, len(d.Embedding))
		for j, f := range d.Embedding {
			vec[j] = float32(f)
		}
		vectors[i] = vec
	}
	return vectors, nil
}

func isRetryableNetErr(err error) bool {
	// net errors like timeouts
	var nerr net.Error
	if errors.As(err, &nerr) {
		if nerr.Timeout() {
			return true
		}
	}
	// EOF or connection reset
	if errors.Is(err, io.EOF) {
		return true
	}
	return false
}

// parseRetryAfterSeconds tries to interpret Retry-After header value as seconds or HTTP date.
func parseRetryAfterSeconds(v string) (int, error) {
	// Try integer seconds first
	if s, err := strconv.Atoi(v); err == nil {
		return s, nil
	}
	// Try HTTP-date
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return int(d.Seconds()), nil
	}
	return 0, fmt.Errorf("invalid Retry-After: %q", v)
}

// classifyAPIError maps generic APIError to typed errors for better UX.
func classifyAPIError(apiErr *APIError, resp *http.Response) error {
	sc := apiErr.StatusCode
	msg := apiErr.Message
	code := apiErr.Code
	// Auth
	if sc == http.StatusUnauthorized || sc == http.StatusForbidden {
		return &AuthError{APIError: apiErr}
	}
	// Rate limiting
	if sc == http.StatusTooManyRequests {
		var ra time.Duration
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := parseRetryAfterSeconds(v); err == nil && secs > 0 {
				ra = time.Duration(secs) * time.Second
			}
		}
		return &RateLimitError{APIError: apiErr, RetryAfter: ra}
	}
	// Not found -> model not found if message/code suggests it
	if sc == http.StatusNotFound {
		if code == "model_not_found" || containsAllFold(msg, "model", "not", "found") {
			return &ModelNotFoundError{APIError: apiErr}
		}
		return apiErr
	}
	// Bad request
	if sc == http.StatusBadRequest {
		return &BadRequestError{APIError: apiErr}
	}
	// Quota/billing signals (heuristic)
	if code == "quota_exceeded" || containsAnyFold(msg, "quota", "billing", "limit exceeded") {
		return &QuotaExceededError{APIError: apiErr}
	}
	// Server errors
	if sc >= 500 && sc <= 599 {
		return &ServerError{APIError: apiErr}
	}
	return apiErr
}

func containsAllFold(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsFold(s, sub) {
			return false
		}
	}
	return true
}

func containsAnyFold(s string, subs ...string) bool {
	for _, sub := range subs {
		if containsFold(s, sub) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if s == "" || sub == "" {
		return false
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// extractRequestID pulls a best-effort request ID from common headers.
func extractRequestID(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	// Common variants
	keys := []string{"X-Request-Id", "X-Request-ID", "OpenAI-Request-ID", "Openrouter-Request-ID", "X-Amzn-Requestid"}
	for _, k := range keys {
		if v := resp.Header.Get(k); v != "" {
			return v
		}
	}
	return ""
}

// withJitter returns a backoff duration with +/- 20% jitter applied.
func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 500 * time.Millisecond
	}
	// jitter factor in [0.8, 1.2)
	f := 0.8 + rand.Float64()*0.4
	out := time.Duration(float64(d) * f)
	if out <= 0 {
		return d
	}
	return out
}

// GenerateStream streams content using OpenRouter's SSE-compatible stream.
// onDelta is called for each partial content chunk.
func (c *Client) GenerateStream(ctx context.Context, req GenerateRequest, onDelta func(string)) error {
	if c.apiKey == "" {
		return errors.New("OPENROUTER_API_KEY is missing")
	}
	if err := c.ValidateModel(req.Model); err != nil {
		return err
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	endpoint := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/KaramelBytes/smushmux")
	httpReq.Header.Set("X-Title", "SmushMux CLI")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		var raw map[string]any
		_ = json.Unmarshal(body, &raw)
		apiErr := &APIError{StatusCode: resp.StatusCode, Raw: raw}
		if v, ok := raw["error"].(map[string]any); ok {
			if msg, ok := v["message"].(string); ok {
				apiErr.Message = msg
			}
			if code, ok := v["code"].(string); ok {
				apiErr.Code = code
			}
		}
		return classifyAPIError(apiErr, resp)
	}
	type streamDelta struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1<<20)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var d streamDelta
			if err := json.Unmarshal([]byte(data), &d); err == nil {
				if len(d.Choices) > 0 {
					onDelta(d.Choices[0].Delta.Content)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read: %w", err)
	}
	return nil
}
