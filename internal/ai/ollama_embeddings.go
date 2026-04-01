package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OllamaEmbClient struct {
	httpClient *http.Client
	host       string
}

func NewOllamaEmbClient(host string, timeout time.Duration) *OllamaEmbClient {
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OllamaEmbClient{httpClient: &http.Client{Timeout: timeout}, host: host}
}

// Embed requests embeddings for a batch of inputs using Ollama's /api/embeddings endpoint.
// Ollama currently accepts single prompts per call; we loop inputs.
func (c *OllamaEmbClient) Embed(ctx context.Context, model string, inputs []string) ([][]float32, error) {
	type reqBody struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	type respBody struct {
		Embedding []float64 `json:"embedding"`
	}
	out := make([][]float32, 0, len(inputs))
	for _, s := range inputs {
		b, _ := json.Marshal(reqBody{Model: model, Prompt: s})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/embeddings", bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
				err = fmt.Errorf("ollama embeddings status %s: %s", resp.Status, string(body))
				return
			}
			var rb respBody
			if decErr := json.NewDecoder(resp.Body).Decode(&rb); decErr != nil {
				err = fmt.Errorf("decode: %w", decErr)
				return
			}
			vec := make([]float32, len(rb.Embedding))
			for i := range rb.Embedding {
				vec[i] = float32(rb.Embedding[i])
			}
			out = append(out, vec)
		}()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
