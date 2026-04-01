package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
)

type stubRuntime struct{}

func (stubRuntime) Generate(context.Context, ai.GenerateRequest) (*ai.GenerateResponse, error) {
	return nil, nil
}

type stubStreamRuntime struct {
	called int
	err    error
}

func (s *stubStreamRuntime) Generate(context.Context, ai.GenerateRequest) (*ai.GenerateResponse, error) {
	return nil, nil
}

func (s *stubStreamRuntime) GenerateStream(ctx context.Context, req ai.GenerateRequest, onDelta func(string)) error {
	s.called++
	onDelta("chunk")
	return s.err
}

func TestSelectModelPrecedence(t *testing.T) {
	cfg := &cfgpkg.Global{DefaultModel: "cfg-model"}
	p := &project.Project{Config: &project.ProjectConfig{Model: "project-model"}}

	if got := selectModel(p, cfg, "cli-model"); got != "cli-model" {
		t.Fatalf("expected CLI model, got %q", got)
	}
	if got := selectModel(p, cfg, ""); got != "project-model" {
		t.Fatalf("expected project model, got %q", got)
	}
	p.Config.Model = ""
	if got := selectModel(p, cfg, ""); got != "cfg-model" {
		t.Fatalf("expected config model, got %q", got)
	}
	cfg.DefaultModel = ""
	if got := selectModel(p, cfg, ""); got != "openai/gpt-4o-mini" {
		t.Fatalf("expected fallback model, got %q", got)
	}
}

func TestEnforceBudget(t *testing.T) {
	if err := enforceBudget(0.0, 1.0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := enforceBudget(2.0, 1.0); err == nil {
		t.Fatal("expected error when cost exceeds budget")
	}
}

func TestHandleStreamingHappyPath(t *testing.T) {
	runtime := &stubStreamRuntime{}
	buf := &bytes.Buffer{}
	delta := &bytes.Buffer{}

	handled, err := handleStreaming(context.Background(), runtime, ai.GenerateRequest{}, streamingOptions{
		Enabled:     true,
		Quiet:       false,
		PrintPrompt: true,
		Prompt:      "example",
		Writer:      buf,
		DeltaWriter: delta,
	})
	if err != nil {
		t.Fatalf("handleStreaming returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected streaming to be handled")
	}
	if runtime.called != 1 {
		t.Fatalf("expected stream runtime to be invoked once, got %d", runtime.called)
	}
	if got := delta.String(); !strings.Contains(got, "chunk") {
		t.Fatalf("expected delta output, got %q", got)
	}
	if out := buf.String(); !strings.Contains(out, "(streaming)") {
		t.Fatalf("expected streaming log output, got %q", out)
	}
}

func TestHandleStreamingFallback(t *testing.T) {
	buf := &bytes.Buffer{}
	handled, err := handleStreaming(context.Background(), stubRuntime{}, ai.GenerateRequest{}, streamingOptions{
		Enabled: true,
		Quiet:   false,
		Writer:  buf,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected fallback to non-streaming")
	}
	if out := buf.String(); !strings.Contains(out, "Streaming not supported") {
		t.Fatalf("expected fallback message, got %q", out)
	}
}

func TestPrepareRetrievedContextInsertsSection(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	p := &project.Project{
		Instructions: "Use retrieved context",
		Documents: map[string]*project.Document{
			"doc1": {ID: "doc1", Name: "doc.txt", Content: "content"},
		},
	}
	prompt, tokens, err := p.BuildPrompt()
	if err != nil {
		t.Fatalf("BuildPrompt failed: %v", err)
	}

	var capturedProvider, capturedModel string
	deps := retrievalDeps{
		newEmbedder: func(ctx context.Context, provider, model string, cfg *cfgpkg.Global, opts retrievalOptions) (retrieval.Embedder, error) {
			capturedProvider = provider
			capturedModel = model
			return embedFunc(func(context.Context, []string) ([][]float32, error) {
				return [][]float32{{1}}, nil
			}), nil
		},
		buildIndex: func(ctx context.Context, emb retrieval.Embedder, root string, docs map[string]struct{ Name, Content string }, opts retrieval.BuildOptions) (*retrieval.Index, error) {
			if len(docs) != 1 {
				t.Fatalf("expected one doc, got %d", len(docs))
			}
			return &retrieval.Index{Records: []retrieval.Record{{DocID: "doc1", DocName: "doc.txt", ChunkID: 0, Text: "retrieved", Vector: []float32{1}}}}, nil
		},
	}

	cfg := &cfgpkg.Global{EmbeddingProvider: "ollama", EmbeddingModel: "custom-embed"}
	prompt2, tokens2, _, err := prepareRetrievedContext(context.Background(), p, prompt, tokens, cfg, retrievalOptions{Enabled: true}, deps)
	if err != nil {
		t.Fatalf("prepareRetrievedContext failed: %v", err)
	}
	if tokens2 == tokens {
		t.Fatalf("expected tokens to change after retrieval")
	}
	if !strings.Contains(prompt2, "[RETRIEVED CONTEXT]") || !strings.Contains(prompt2, "retrieved") {
		t.Fatalf("retrieved context not inserted: %q", prompt2)
	}
	if capturedProvider != ai.ProviderOllama {
		t.Fatalf("expected provider %q, got %q", ai.ProviderOllama, capturedProvider)
	}
	if capturedModel != "custom-embed" {
		t.Fatalf("expected model %q, got %q", "custom-embed", capturedModel)
	}
}

func TestPrepareRetrievedContextDisabled(t *testing.T) {
	p := &project.Project{
		Instructions: "No retrieval",
		Documents: map[string]*project.Document{
			"doc1": {ID: "doc1", Name: "doc.txt", Content: "content"},
		},
	}
	prompt, tokens, err := p.BuildPrompt()
	if err != nil {
		t.Fatalf("BuildPrompt failed: %v", err)
	}
	prompt2, tokens2, _, err := prepareRetrievedContext(context.Background(), p, prompt, tokens, nil, retrievalOptions{Enabled: false}, retrievalDeps{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt2 != prompt || tokens2 != tokens {
		t.Fatalf("expected prompt unchanged; got prompt=%q tokens=%d", prompt2, tokens2)
	}
}

func TestBuildRuntimeDefaults(t *testing.T) {
	cfg := &cfgpkg.Global{DefaultProvider: "local", OllamaHost: "http://example"}
	client, provider, err := buildRuntime(cfg, runtimeOptions{})
	if err != nil {
		t.Fatalf("buildRuntime error: %v", err)
	}
	if provider != ai.ProviderOllama {
		t.Fatalf("expected ollama provider, got %q", provider)
	}
	if client == nil {
		t.Fatal("expected runtime client")
	}
}

func TestFormatAndWriteOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	buf := &bytes.Buffer{}
	if err := formatAndWriteOutput("content", outputOptions{
		JSON:         false,
		Quiet:        false,
		Project:      "proj",
		Model:        "model",
		MaxTokens:    10,
		Temperature:  0.5,
		PromptTokens: 4,
		OutputPath:   path,
		OutputFormat: "text",
		Writer:       buf,
	}); err != nil {
		t.Fatalf("formatAndWriteOutput error: %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "=== AI Response ===") {
		t.Fatalf("expected formatted output, got %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestHandleStreamingErrorPropagation(t *testing.T) {
	runtime := &stubStreamRuntime{err: errors.New("fail")}
	handled, err := handleStreaming(context.Background(), runtime, ai.GenerateRequest{}, streamingOptions{
		Enabled:     true,
		Quiet:       true,
		DeltaWriter: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error from streaming runtime")
	}
	if !handled {
		t.Fatal("expected handled to be true even on error")
	}
}
