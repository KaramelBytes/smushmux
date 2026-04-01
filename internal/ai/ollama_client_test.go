package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestOllamaGenerateSuccess(t *testing.T) {
	srv := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{"role": "assistant", "content": "hello from ollama"},
		})
	}))
	defer srv.Close()

	c := NewOllamaClient(srv.URL, 2*time.Second, 1, 0, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := c.Generate(ctx, GenerateRequest{Model: "llama3:latest", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 16})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "hello from ollama" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.RequestID == "" {
		t.Fatalf("expected simulated request id")
	}
}

func TestOllamaGenerateBadRequest(t *testing.T) {
	srv := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "bad request"})
	}))
	defer srv.Close()
	c := NewOllamaClient(srv.URL, 2*time.Second, 1, 0, 0)
	_, err := c.Generate(context.Background(), GenerateRequest{Model: "llama3:latest", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestOllamaGenerateEmptyMessages(t *testing.T) {
	c := NewOllamaClient("http://localhost:11434", 2*time.Second, 1, 0, 0)

	// Test empty messages slice
	_, err := c.Generate(context.Background(), GenerateRequest{Model: "llama3:latest", Messages: []Message{}})
	if err == nil || err.Error() != "messages cannot be empty" {
		t.Fatalf("expected 'messages cannot be empty' error, got: %v", err)
	}

	// Test GenerateStream with empty messages
	err = c.GenerateStream(context.Background(), GenerateRequest{Model: "llama3:latest", Messages: []Message{}}, func(string) {})
	if err == nil || err.Error() != "messages cannot be empty" {
		t.Fatalf("expected 'messages cannot be empty' error, got: %v", err)
	}
}

func TestOllamaGenerateMultipleMessages(t *testing.T) {
	// Capture the request to verify all messages are preserved
	var capturedRequest ollamaChatRequest
	srv := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}

		// Decode the request to verify message handling
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{"role": "assistant", "content": "response"},
		})
	}))
	defer srv.Close()

	c := NewOllamaClient(srv.URL, 2*time.Second, 1, 0, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test with multiple messages including system and user roles
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	_, err := c.Generate(ctx, GenerateRequest{Model: "llama3:latest", Messages: messages})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Verify all messages were preserved in the request
	if len(capturedRequest.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(capturedRequest.Messages))
	}

	expectedMessages := []ollamaChatMessage{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	for i, expected := range expectedMessages {
		if capturedRequest.Messages[i].Role != expected.Role {
			t.Fatalf("message %d role: expected %s, got %s", i, expected.Role, capturedRequest.Messages[i].Role)
		}
		if capturedRequest.Messages[i].Content != expected.Content {
			t.Fatalf("message %d content: expected %s, got %s", i, expected.Content, capturedRequest.Messages[i].Content)
		}
	}
}
