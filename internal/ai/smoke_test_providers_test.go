// +build !race

package ai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestSmokeOllamaBasicCompletion tests basic completion with Ollama.
// This test is only run when explicitly enabled via SMOKE_TEST_OLLAMA env var.
// Requires a local Ollama instance running on http://localhost:11434.
func TestSmokeOllamaBasicCompletion(t *testing.T) {
	if os.Getenv("SMOKE_TEST_OLLAMA") != "true" {
		t.Skip("SMOKE_TEST_OLLAMA env var not set; skipping Ollama smoke test")
	}

	ollamaBase := "http://localhost:11434"
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "tinyllama"
	}

	// Verify Ollama is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", ollamaBase+"/api/tags", nil)
		if err != nil {
			t.Logf("Attempt %d: Failed to create request: %v", attempt+1, err)
			lastErr = err
			time.Sleep(time.Second)
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("Attempt %d: Ollama not reachable: %v", attempt+1, err)
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Logf("Ollama is reachable at %s", ollamaBase)
			lastErr = nil
			break
		}

		t.Logf("Attempt %d: Ollama returned status %d", attempt+1, resp.StatusCode)
		lastErr = fmt.Errorf("status %d", resp.StatusCode)
		time.Sleep(time.Second)
	}

	if lastErr != nil {
		t.Skipf("Could not reach Ollama after retries: %v", lastErr)
	}

	// Test: Basic completion via Ollama API
	genReq := map[string]interface{}{
		"model":  modelName,
		"prompt": "Write a haiku about code in 5 words or less:",
		"stream": false,
	}

	reqBody, err := json.Marshal(genReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaBase+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to call Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var genResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	response, ok := genResp["response"].(string)
	if !ok || response == "" {
		t.Fatalf("Empty response from Ollama: %v", genResp)
	}

	// Verify response contains expected content
	if len(response) < 10 {
		t.Logf("Warning: short response (%d chars)", len(response))
	}

	tokens, _ := genResp["eval_count"].(float64)
	promptTokens, _ := genResp["prompt_eval_count"].(float64)

	t.Logf("✓ Ollama %s completed successfully", modelName)
	t.Logf("  Prompt tokens: %.0f | Generated tokens: %.0f", promptTokens, tokens)
	t.Logf("  Response: %s...", truncate(response, 60))
}

// TestSmokeOpenRouterFreeTier tests basic completion with OpenRouter free-tier.
// This test is only run when explicitly enabled via SMOKE_TEST_OPENROUTER env var.
// Requires OPENROUTER_API_KEY secret to be set.
func TestSmokeOpenRouterFreeTier(t *testing.T) {
	if os.Getenv("SMOKE_TEST_OPENROUTER") != "true" {
		t.Skip("SMOKE_TEST_OPENROUTER env var not set; skipping OpenRouter smoke test")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set; skipping OpenRouter smoke test")
	}

	// Use a free-tier friendly model with strict token budget
	modelID := "mistralai/mistral-7b-instruct"
	maxTokens := 50 // Very short to stay within free tier budget

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a basic completion request via OpenRouter
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":       modelID,
		"messages":    []map[string]string{{"role": "user", "content": "Classify this as positive or negative: 'The weather is nice today.' Answer with one word only."}},
		"max_tokens":  maxTokens,
		"temperature": 0.3,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/jeremiahhenning/smushmux")
	req.Header.Set("X-Title", "smushmux-smoke-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Could not reach OpenRouter: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Parse response
	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		t.Logf("Response body: %s", string(body))
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check for errors
	if errVal, hasErr := respData["error"]; hasErr {
		errMsg := fmt.Sprintf("%v", errVal)
		if resp.StatusCode == http.StatusUnauthorized {
			t.Skipf("OpenRouter API key invalid or expired: %s", errMsg)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Skipf("OpenRouter rate limit exceeded: %s", errMsg)
		}
		t.Fatalf("OpenRouter error (status %d): %s", resp.StatusCode, errMsg)
	}

	// Verify response structure
	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("Invalid or empty choices in response: %v", respData)
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid choice structure: %v", choices[0])
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid message structure: %v", choice)
	}

	content, ok := message["content"].(string)
	if !ok || content == "" {
		t.Fatalf("Empty or invalid content: %v", message)
	}

	// Extract token usage
	usage, _ := respData["usage"].(map[string]interface{})
	promptTokens, _ := usage["prompt_tokens"].(float64)
	completionTokens, _ := usage["completion_tokens"].(float64)
	totalTokens, _ := usage["total_tokens"].(float64)

	// Sanity check: response should be short (max_tokens=50)
	if completionTokens > 100 {
		t.Logf("Warning: completion tokens (%d) exceeded expected max", int(completionTokens))
	}

	t.Logf("✓ OpenRouter %s completed successfully", modelID)
	t.Logf("  Tokens: prompt=%.0f | completion=%.0f | total=%.0f", promptTokens, completionTokens, totalTokens)
	t.Logf("  Response: %s...", truncate(content, 60))
}

// truncate returns a truncated string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
