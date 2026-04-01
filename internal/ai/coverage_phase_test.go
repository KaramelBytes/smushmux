package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"io"
)

func TestCatalogHelpersAndCostEstimation(t *testing.T) {
	orig := Catalog()
	t.Cleanup(func() { OverrideCatalog(orig) })

	if _, ok := LookupModel("openai/gpt-4o-mini"); !ok {
		t.Fatalf("expected known model lookup success")
	}
	if _, ok := LookupModel("does/not-exist"); ok {
		t.Fatalf("expected unknown model lookup failure")
	}

	if cost, ok := EstimateCostUSD("openai/gpt-4o-mini", 1000, 1000); !ok || cost <= 0 {
		t.Fatalf("expected positive known-model estimate, got cost=%f ok=%v", cost, ok)
	}
	if _, ok := EstimateCostUSD("unknown/model", 1000, 1000); ok {
		t.Fatalf("expected unknown-model estimate to return ok=false")
	}

	OverrideCatalog(map[string]ModelInfo{"a": {Name: "a", ContextTokens: 10}})
	MergeCatalog(map[string]ModelInfo{"b": {Name: "b", ContextTokens: 20}})
	cat := Catalog()
	if _, ok := cat["a"]; !ok {
		t.Fatalf("expected overridden model a")
	}
	if _, ok := cat["b"]; !ok {
		t.Fatalf("expected merged model b")
	}

	cat["a"] = ModelInfo{Name: "mutated"}
	cat2 := Catalog()
	if cat2["a"].Name == "mutated" {
		t.Fatalf("Catalog should return a copy, not shared map")
	}
}

func TestLoadCatalogFromJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "models.json")
	if err := os.WriteFile(p, []byte(`{"x/model":{"Name":"x/model","ContextTokens":123,"InputPerK":0.1,"OutputPerK":0.2}}`), 0o644); err != nil {
		t.Fatalf("write models json: %v", err)
	}
	m, err := LoadCatalogFromJSON(p)
	if err != nil {
		t.Fatalf("load catalog json: %v", err)
	}
	if _, ok := m["x/model"]; !ok {
		t.Fatalf("expected x/model entry")
	}

	if _, err := LoadCatalogFromJSON(filepath.Join(d, "missing.json")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestRuntimeRegistryAndDefaults(t *testing.T) {
	if rt, ok := GetRuntime("does-not-exist", RuntimeConfig{}); ok || rt != nil {
		t.Fatalf("expected unknown provider to return nil,false")
	}

	rt, ok := GetRuntime(ProviderOpenRouter, RuntimeConfig{})
	if !ok {
		t.Fatalf("expected openrouter runtime to be registered")
	}
	orc, ok := rt.(*Client)
	if !ok {
		t.Fatalf("expected *Client runtime type")
	}
	if orc.retryMaxAttempts != 3 || orc.retryBaseDelay != 500*time.Millisecond || orc.retryMaxDelay != 4*time.Second {
		t.Fatalf("unexpected openrouter defaults: retry=%d base=%v max=%v", orc.retryMaxAttempts, orc.retryBaseDelay, orc.retryMaxDelay)
	}

	rt, ok = GetRuntime(ProviderOllama, RuntimeConfig{})
	if !ok {
		t.Fatalf("expected ollama runtime to be registered")
	}
	ol, ok := rt.(*OllamaClient)
	if !ok {
		t.Fatalf("expected *OllamaClient runtime type")
	}
	if ol.host == "" || ol.retryMaxAttempts != 2 || ol.retryBaseDelay != 200*time.Millisecond || ol.retryMaxDelay != time.Second {
		t.Fatalf("unexpected ollama defaults")
	}
}

func TestHelperFunctionsAndErrorClassification(t *testing.T) {
	if containsFold("", "x") || containsFold("abc", "") {
		t.Fatalf("containsFold empty behavior mismatch")
	}
	if !containsFold("AbC", "bc") {
		t.Fatalf("containsFold case-insensitive match expected")
	}
	if !containsAllFold("model not found", "model", "found") {
		t.Fatalf("containsAllFold expected true")
	}
	if !containsAnyFold("quota exceeded", "billing", "quota") {
		t.Fatalf("containsAnyFold expected true")
	}

	if _, err := parseRetryAfterSeconds("abc"); err == nil {
		t.Fatalf("expected parseRetryAfterSeconds error for invalid value")
	}
	if s, err := parseRetryAfterSeconds("2"); err != nil || s != 2 {
		t.Fatalf("expected parseRetryAfterSeconds int parse success, got s=%d err=%v", s, err)
	}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("X-Request-Id", "req123")
	if got := extractRequestID(resp); got != "req123" {
		t.Fatalf("request id extraction mismatch: %q", got)
	}

	apiErr := &APIError{StatusCode: http.StatusUnauthorized, Message: "bad key"}
	if _, ok := classifyAPIError(apiErr, &http.Response{Header: make(http.Header)}).(*AuthError); !ok {
		t.Fatalf("expected AuthError")
	}
	apiErr = &APIError{StatusCode: http.StatusTooManyRequests, Message: "slow down"}
	resp429 := &http.Response{Header: make(http.Header)}
	resp429.Header.Set("Retry-After", "1")
	if _, ok := classifyAPIError(apiErr, resp429).(*RateLimitError); !ok {
		t.Fatalf("expected RateLimitError")
	}
	apiErr = &APIError{StatusCode: http.StatusNotFound, Message: "model not found", Code: "model_not_found"}
	if _, ok := classifyAPIError(apiErr, &http.Response{Header: make(http.Header)}).(*ModelNotFoundError); !ok {
		t.Fatalf("expected ModelNotFoundError")
	}
	apiErr = &APIError{StatusCode: http.StatusBadRequest, Message: "bad request"}
	if _, ok := classifyAPIError(apiErr, &http.Response{Header: make(http.Header)}).(*BadRequestError); !ok {
		t.Fatalf("expected BadRequestError")
	}
	apiErr = &APIError{StatusCode: http.StatusInternalServerError, Message: "oops"}
	if _, ok := classifyAPIError(apiErr, &http.Response{Header: make(http.Header)}).(*ServerError); !ok {
		t.Fatalf("expected ServerError")
	}

	if got := (&AuthError{APIError: &APIError{StatusCode: 401, Message: "x"}}).Error(); !strings.Contains(got, "authentication failed") {
		t.Fatalf("unexpected AuthError message: %q", got)
	}
	if got := (&QuotaExceededError{APIError: &APIError{StatusCode: 429, Message: "x"}}).Error(); !strings.Contains(got, "quota exceeded") {
		t.Fatalf("unexpected QuotaExceededError message: %q", got)
	}
	if got := (&UnreachableError{Host: "http://x", Err: errors.New("dial failed")}).Error(); !strings.Contains(got, "endpoint unreachable") {
		t.Fatalf("unexpected UnreachableError message: %q", got)
	}
}

type timeoutNetErr struct{}

func (timeoutNetErr) Error() string   { return "timeout" }
func (timeoutNetErr) Timeout() bool   { return true }
func (timeoutNetErr) Temporary() bool { return true }

func TestIsRetryableNetErr(t *testing.T) {
	if !isRetryableNetErr(timeoutNetErr{}) {
		t.Fatalf("expected timeout net error to be retryable")
	}
	if !isRetryableNetErr(ioEOFErr()) {
		t.Fatalf("expected io.EOF to be retryable")
	}
	if isRetryableNetErr(errors.New("plain error")) {
		t.Fatalf("expected plain error to be non-retryable")
	}
}

func ioEOFErr() error {
	return fmt.Errorf("wrapped: %w", io.EOF)
}

func TestNewOpenRouterClientDefaults(t *testing.T) {
	c := NewOpenRouterClient("k")
	if c == nil || c.apiKey != "k" {
		t.Fatalf("expected initialized client")
	}
	if c.retryMaxAttempts != 3 || c.retryBaseDelay != 500*time.Millisecond || c.retryMaxDelay != 4*time.Second {
		t.Fatalf("unexpected default retry config")
	}
}

func TestOllamaEmbClientEmbed(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/embeddings" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"embedding":[0.1,0.2,0.3]}`)
	}))
	defer ts.Close()

	c := NewOllamaEmbClient(ts.URL, time.Second)
	vecs, err := c.Embed(context.Background(), "nomic-embed-text", []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed success case: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 {
		t.Fatalf("unexpected vectors shape: %v", vecs)
	}
	if requests != 2 {
		t.Fatalf("expected one request per input, got %d", requests)
	}
}

func TestOllamaEmbClientEmbedNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	defer ts.Close()

	c := NewOllamaEmbClient(ts.URL, time.Second)
	if _, err := c.Embed(context.Background(), "m", []string{"x"}); err == nil {
		t.Fatalf("expected non-200 error")
	}
}

func TestClientEmbedValidationAndSuccess(t *testing.T) {
	c := NewClient("", time.Second, 1, time.Millisecond, 2*time.Millisecond)
	if _, err := c.Embed(context.Background(), "m", []string{"x"}); err == nil {
		t.Fatalf("expected missing api key error")
	}

	c = NewClient("k", time.Second, 1, time.Millisecond, 2*time.Millisecond)
	if _, err := c.Embed(context.Background(), "", []string{"x"}); err == nil {
		t.Fatalf("expected empty model error")
	}
	if _, err := c.Embed(context.Background(), "m", nil); err == nil {
		t.Fatalf("expected empty inputs error")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"index":0,"embedding":[0.1,0.2]},{"index":1,"embedding":[0.3]}]}`)
	}))
	defer ts.Close()

	c = NewClientWithBaseURL("k", time.Second, 1, time.Millisecond, 2*time.Millisecond, ts.URL)
	vecs, err := c.Embed(context.Background(), "embed-model", []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed success: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 2 || len(vecs[1]) != 1 {
		t.Fatalf("unexpected vectors shape: %#v", vecs)
	}
}

func TestClientEmbedClassifiedAndDecodeErrors(t *testing.T) {
	ts401 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"bad key","code":"unauthorized"}}`)
	}))
	defer ts401.Close()

	c := NewClientWithBaseURL("k", time.Second, 1, time.Millisecond, 2*time.Millisecond, ts401.URL)
	if _, err := c.Embed(context.Background(), "m", []string{"x"}); err == nil {
		t.Fatalf("expected auth error")
	} else {
		if _, ok := err.(*AuthError); !ok {
			t.Fatalf("expected *AuthError, got %T (%v)", err, err)
		}
	}

	tsBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"data":`) // invalid JSON
	}))
	defer tsBadJSON.Close()

	c = NewClientWithBaseURL("k", time.Second, 1, time.Millisecond, 2*time.Millisecond, tsBadJSON.URL)
	if _, err := c.Embed(context.Background(), "m", []string{"x"}); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestGenerateStreamMalformedEventAndClassifiedError(t *testing.T) {
	var out string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {not-json}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("k", time.Second, 1, time.Millisecond, 2*time.Millisecond, ts.URL)
	if err := c.GenerateStream(context.Background(), GenerateRequest{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}, func(d string) {
		out += d
	}); err != nil {
		t.Fatalf("expected malformed events to be ignored, got error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("expected only valid delta emitted, got %q", out)
	}

	ts404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":{"message":"model not found","code":"model_not_found"}}`)
	}))
	defer ts404.Close()
	c = NewClientWithBaseURL("k", time.Second, 1, time.Millisecond, 2*time.Millisecond, ts404.URL)
	if err := c.GenerateStream(context.Background(), GenerateRequest{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}, func(string) {}); err == nil {
		t.Fatalf("expected classified error")
	} else if _, ok := err.(*ModelNotFoundError); !ok {
		t.Fatalf("expected *ModelNotFoundError, got %T (%v)", err, err)
	}
}
