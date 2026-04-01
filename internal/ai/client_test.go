package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type ipv4Server struct {
	URL string
	srv *http.Server
	ln  net.Listener
}

func newIPv4Server(t *testing.T, handler http.Handler) *ipv4Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
			t.Skipf("skipping test: cannot open local listener (%v)", err)
		}
		t.Fatalf("listen tcp4: %v", err)
	}
	srv := &http.Server{Handler: handler}
	s := &ipv4Server{
		URL: "http://" + ln.Addr().String(),
		srv: srv,
		ln:  ln,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("test server serve: %v", err))
		}
	}()
	return s
}

func (s *ipv4Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func testServerSequence(t *testing.T, statuses []int, headers []http.Header, bodyOK any) *ipv4Server {
	t.Helper()
	var idx int32
	return newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		i := int(atomic.AddInt32(&idx, 1)) - 1
		if i >= len(statuses) {
			i = len(statuses) - 1
		}
		st := statuses[i]
		if headers != nil && i < len(headers) && headers[i] != nil {
			for k, vals := range headers[i] {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
		}
		if st >= 200 && st < 300 {
			w.WriteHeader(st)
			_ = json.NewEncoder(w).Encode(bodyOK)
			return
		}
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "rate limited"}})
	}))
}

func TestGenerateRetriesOn429(t *testing.T) {
	okBody := GenerateResponse{Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}}}
	srv := testServerSequence(t, []int{429, 200}, []http.Header{{"Retry-After": {"0"}}, {}}, okBody)
	defer srv.Close()

	c := NewClientWithBaseURL("test", 2*time.Second, 3, 10*time.Millisecond, 100*time.Millisecond, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.Generate(ctx, GenerateRequest{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 1})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRetryAfterHonored(t *testing.T) {
	okBody := GenerateResponse{Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}}}
	// Ask server to instruct a 1-second Retry-After, then succeed.
	srv := testServerSequence(t, []int{429, 200}, []http.Header{{"Retry-After": {"1"}}, {}}, okBody)
	defer srv.Close()

	c := NewClientWithBaseURL("test", 5*time.Second, 3, 0, 0, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	_, err := c.Generate(ctx, GenerateRequest{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 1})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 900*time.Millisecond { // allow some scheduling variance
		t.Fatalf("expected at least ~1s delay due to Retry-After, got %v", elapsed)
	}
}

func TestErrorIncludesRequestID(t *testing.T) {
	// Server returns 400 with X-Request-Id header
	srv := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Request-Id", "req_test_123")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad req", "code": "bad_request"}})
	}))
	defer srv.Close()

	c := NewClientWithBaseURL("test", 2*time.Second, 1, 10*time.Millisecond, 50*time.Millisecond, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.Generate(ctx, GenerateRequest{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "req_test_123") {
		t.Fatalf("expected request id in error, got: %v", err)
	}
}

func TestOpenRouterStreamParsesDeltas(t *testing.T) {
	srv := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// Send two delta events then DONE
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClientWithBaseURL("test", 5*time.Second, 1, 0, 0, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var out string
	err := c.GenerateStream(ctx, GenerateRequest{Model: "test", Messages: []Message{{Role: "user", Content: "hi"}}}, func(d string) { out += d })
	if err != nil {
		t.Fatalf("GenerateStream error: %v", err)
	}
	if out != "hello world" {
		t.Fatalf("unexpected stream accumulation: %q", out)
	}
}
