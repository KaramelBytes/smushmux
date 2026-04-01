package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()
	_ = w.Close()
	os.Stdout = orig
	outBytes, _ := io.ReadAll(r)
	_ = r.Close()
	return string(outBytes), runErr
}

func TestConfigShowNoConfig(t *testing.T) {
	preserveCmdGlobals(t)
	cfg = nil

	out, err := captureStdout(t, func() error {
		return configShowCmd.RunE(configShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !strings.Contains(out, "No config loaded") {
		t.Fatalf("expected no-config message, got %q", out)
	}
}

func TestConfigShowMaskedAndFields(t *testing.T) {
	preserveCmdGlobals(t)
	cfg = &cfgpkg.Global{
		APIKey:            "sk-test-abcdef123456",
		DefaultModel:      "openai/gpt-4o-mini",
		DefaultProvider:   "openrouter",
		EmbeddingModel:    "openai/text-embedding-3-small",
		EmbeddingProvider: "openrouter",
		RetrievalTopK:     6,
		RetrievalMinScore: 0.25,
		MaxTokens:         1024,
		Temperature:       0.7,
		ProjectsDir:       "/tmp/projects",
	}

	out, err := captureStdout(t, func() error {
		return configShowCmd.RunE(configShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !strings.Contains(out, "api_key: sk-****456") {
		t.Fatalf("expected masked api key in output, got %q", out)
	}
	if !strings.Contains(out, "default_model: openai/gpt-4o-mini") {
		t.Fatalf("expected default_model in output, got %q", out)
	}
	if !strings.Contains(out, "retrieval_min_score: 0.250") {
		t.Fatalf("expected retrieval_min_score in output, got %q", out)
	}
}

func TestModelsShowOutputsValidJSON(t *testing.T) {
	preserveCmdGlobals(t)
	original := ai.Catalog()
	t.Cleanup(func() { ai.OverrideCatalog(original) })
	ai.OverrideCatalog(map[string]ai.ModelInfo{
		"z-model": {Name: "z-model", ContextTokens: 1000},
		"a-model": {Name: "a-model", ContextTokens: 2000},
	})

	out, err := captureStdout(t, func() error {
		return modelsShowCmd.RunE(modelsShowCmd, nil)
	})
	if err != nil {
		t.Fatalf("models show: %v", err)
	}

	var parsed map[string]ai.ModelInfo
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got err=%v output=%q", err, out)
	}
	if _, ok := parsed["a-model"]; !ok {
		t.Fatalf("expected a-model in output")
	}
}

func TestListAllProjectsOutput(t *testing.T) {
	preserveLifecycleGlobals(t)

	root := t.TempDir()
	cfg = &cfgpkg.Global{ProjectsDir: root}

	out, err := captureStdout(t, func() error {
		return listAllProjects()
	})
	if err != nil {
		t.Fatalf("listAllProjects empty: %v", err)
	}
	if !strings.Contains(out, "(no projects)") {
		t.Fatalf("expected no projects output, got %q", out)
	}

	if err := os.MkdirAll(filepath.Join(root, "p1"), 0o755); err != nil {
		t.Fatalf("mkdir p1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "p1", "project.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write p1 project.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "not-project"), 0o755); err != nil {
		t.Fatalf("mkdir non project dir: %v", err)
	}

	out, err = captureStdout(t, func() error {
		return listAllProjects()
	})
	if err != nil {
		t.Fatalf("listAllProjects populated: %v", err)
	}
	if !strings.Contains(out, "- p1") {
		t.Fatalf("expected p1 in output, got %q", out)
	}
	if strings.Contains(out, "not-project") {
		t.Fatalf("did not expect non-project dir in output, got %q", out)
	}
}
