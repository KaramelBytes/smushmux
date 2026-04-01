package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
)

// runCmd is a helper to execute the root command with args.
func runCmd(t *testing.T, args ...string) {
	t.Helper()
	// Reset sticky flags that may persist Changed state across invocations
	if f := rootCmd.Flags(); f != nil {
		if fl := f.Lookup("budget-limit"); fl != nil {
			_ = fl.Value.Set("0")
			fl.Changed = false
		}
		if fl := f.Lookup("prompt-limit"); fl != nil {
			_ = fl.Value.Set("0")
			fl.Changed = false
		}
		if fl := f.Lookup("print-prompt"); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
	}
	// Reset generateCmd flags as well
	if f := generateCmd.Flags(); f != nil {
		if fl := f.Lookup("budget-limit"); fl != nil {
			_ = fl.Value.Set("0")
			fl.Changed = false
		}
		if fl := f.Lookup("prompt-limit"); fl != nil {
			_ = fl.Value.Set("0")
			fl.Changed = false
		}
		if fl := f.Lookup("print-prompt"); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
		if fl := f.Lookup("dry-run"); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
		if fl := f.Lookup("explain"); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
		if fl := f.Lookup("provider"); fl != nil {
			_ = fl.Value.Set("")
			fl.Changed = false
		}
		if fl := f.Lookup("model"); fl != nil {
			_ = fl.Value.Set("")
			fl.Changed = false
		}
		if fl := f.Lookup("max-tokens"); fl != nil {
			_ = fl.Value.Set("0")
			fl.Changed = false
		}
	}
	// Reset bound variables
	genBudgetLimit = 0
	genPromptLimit = 0
	genPrintPrompt = false
	genDryRun = false
	genExplain = false
	genProvider = ""
	genModel = ""
	genMaxTokens = 0
	genTimeoutSec = 180
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
}

func TestCLI_BudgetLimitBlocksGeneration(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	// Create a larger doc to get a non-trivial token count
	docPath := filepath.Join(home, "doc.md")
	if err := os.WriteFile(docPath, []byte("Title\n\n"+strings.Repeat("content ", 3000)), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	runCmd(t, "init", "budget", "-d", "budget test")
	runCmd(t, "add", "-p", "budget", docPath)
	runCmd(t, "instruct", "-p", "budget", "Summarize")

	// Expect generate to fail due to very small budget
	rootCmd.SetArgs([]string{"generate", "-p", "budget", "--dry-run", "--budget-limit", "0.0001"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error due to budget limit, got nil")
	}
}
func TestCLI_ContextWindowExceededError(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	// Mock the AI client and model info
	ai.MergeCatalog(map[string]ai.ModelInfo{
		"ollama/test-model": {
			Name:          "ollama/test-model",
			ContextTokens: 100,
		},
	})

	// Create a doc file to add
	docPath := filepath.Join(home, "doc1.md")
	if err := os.WriteFile(docPath, []byte(strings.Repeat("a", 4*101)), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	// init project
	runCmd(t, "init", "itest", "-d", "integration test")
	// add doc
	runCmd(t, "add", "-p", "itest", docPath, "--desc", "first doc")
	// set instructions
	runCmd(t, "instruct", "-p", "itest", "Summarize the content")

	// Expect generate to fail due to context window exceeded
	rootCmd.SetArgs([]string{"generate", "-p", "itest", "--provider", "ollama", "--model", "ollama/test-model", "--max-tokens", "50"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error due to context window exceeded, got nil")
	}
}

func TestCLI_Init_Add_Instruct_GenerateDryRun(t *testing.T) {
	// Use a temp HOME to isolate config and projects
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	// Create a doc file to add
	docPath := filepath.Join(home, "doc1.md")
	if err := os.WriteFile(docPath, []byte("# Title\n\nSome content."), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	// init project
	runCmd(t, "init", "itest", "-d", "integration test")
	// add doc
	runCmd(t, "add", "-p", "itest", docPath, "--desc", "first doc")
	// set instructions
	runCmd(t, "instruct", "-p", "itest", "Summarize the content")
	// generate dry-run with prompt limit for speed
	runCmd(t, "generate", "-p", "itest", "--dry-run", "--prompt-limit", "2000")
}

func TestCLI_GenerateDryRunExplain(t *testing.T) {
	// Use a temp HOME to isolate config and projects
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	docPath := filepath.Join(home, "report.md")
	if err := os.WriteFile(docPath, []byte("# Report\n\nFinancial data."), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	runCmd(t, "init", "explaintest", "-d", "explain integration test")
	runCmd(t, "add", "-p", "explaintest", docPath, "--desc", "financial report")
	runCmd(t, "instruct", "-p", "explaintest", "Summarize the financial data")

	// Capture stdout to verify explain output.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	// Reset explain flag state before the test invocation.
	if fl := generateCmd.Flags().Lookup("explain"); fl != nil {
		_ = fl.Value.Set("false")
		fl.Changed = false
	}
	genExplain = false

	rootCmd.SetArgs([]string{"generate", "-p", "explaintest", "--dry-run", "--explain"})
	if execErr := rootCmd.Execute(); execErr != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("command failed: %v", execErr)
	}

	w.Close()
	os.Stdout = origStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	out := buf.String()

	for _, heading := range []string{
		"## Evidence Report",
		"### Project",
		"### Provider / Model",
		"### Prompt Statistics",
		"### Retrieval",
		"### Documents Included in Prompt",
	} {
		if !strings.Contains(out, heading) {
			t.Errorf("explain output missing heading %q\nfull output:\n%s", heading, out)
		}
	}
	if !strings.Contains(out, "explaintest") {
		t.Errorf("explain output does not mention project name 'explaintest'\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "report.md") {
		t.Errorf("explain output does not mention document 'report.md'\nfull output:\n%s", out)
	}
}

func TestCLI_GenerateDryRunExplainRetrieval(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	docPath := filepath.Join(home, "financial.md")
	if err := os.WriteFile(docPath, []byte("# Financials\n\nQ3 revenue increased by 12%."), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	runCmd(t, "init", "retrievaltest", "-d", "retrieval explain test")
	runCmd(t, "add", "-p", "retrievaltest", docPath, "--desc", "financial report")
	runCmd(t, "instruct", "-p", "retrievaltest", "Summarize financial data")

	// Override retrieval deps with a fake embedder so no network calls are made.
	origDeps := defaultRetrievalDeps
	defer func() { defaultRetrievalDeps = origDeps }()
	defaultRetrievalDeps = retrievalDeps{
		newEmbedder: func(ctx context.Context, provider, model string, cfg *cfgpkg.Global, opts retrievalOptions) (retrieval.Embedder, error) {
			return embedFunc(func(context.Context, []string) ([][]float32, error) {
				return [][]float32{{1, 0}}, nil
			}), nil
		},
		buildIndex: func(ctx context.Context, emb retrieval.Embedder, root string, docs map[string]struct{ Name, Content string }, opts retrieval.BuildOptions) (*retrieval.Index, error) {
			return &retrieval.Index{
				Records: []retrieval.Record{
					{DocID: "d1", DocName: "financial.md", ChunkID: 0, Text: "Q3 revenue increased by 12%.", Vector: []float32{1, 0}},
				},
			}, nil
		},
	}

	// Capture stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	// Reset relevant flags.
	for _, name := range []string{"explain", "dry-run", "retrieval"} {
		if fl := generateCmd.Flags().Lookup(name); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
	}
	genExplain = false
	genDryRun = false
	genRetrieval = false

	rootCmd.SetArgs([]string{"generate", "-p", "retrievaltest", "--dry-run", "--explain", "--retrieval"})
	if execErr := rootCmd.Execute(); execErr != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("command failed: %v", execErr)
	}

	w.Close()
	os.Stdout = origStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "## Retrieval evidence") {
		t.Errorf("expected '## Retrieval evidence' section in explain output\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "financial.md") {
		t.Errorf("expected doc name 'financial.md' in retrieval evidence\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "Q3 revenue") {
		t.Errorf("expected chunk preview in retrieval evidence\nfull output:\n%s", out)
	}
}

// fixtureDir returns the absolute path to the test/fixtures directory,
// resolved relative to this source file so it works on any OS.
func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile is cmd/integration_test.go; go up one level to repo root then into test/fixtures
	return filepath.Join(filepath.Dir(thisFile), "..", "test", "fixtures")
}

// TestCLI_FixtureProjectExplain loads the pre-built offline fixture project and
// verifies that "generate --dry-run --explain" succeeds with no network calls and
// produces the expected Evidence Report sections.
func TestCLI_FixtureProjectExplain(t *testing.T) {
	// Use a temp HOME to isolate the projects directory.
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	// Build the project directory path that resolveProjectDirByName will use.
	projDir := filepath.Join(home, ".smushmux", "projects", "evidence_mode_project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	// Read the fixture project.json and patch document paths to absolute fixture paths.
	fixtureSrc := filepath.Join(fixtureDir(t), "evidence_mode_project")
	rawJSON, err := os.ReadFile(filepath.Join(fixtureSrc, "project.json"))
	if err != nil {
		t.Fatalf("read fixture project.json: %v", err)
	}

	// Unmarshal into a generic map so we can update paths portably.
	var proj map[string]any
	if err := json.Unmarshal(rawJSON, &proj); err != nil {
		t.Fatalf("unmarshal fixture project.json: %v", err)
	}
	if docs, ok := proj["documents"].(map[string]any); ok {
		for _, v := range docs {
			if doc, ok := v.(map[string]any); ok {
				if name, ok := doc["name"].(string); ok {
					doc["path"] = filepath.Join(fixtureSrc, name)
				}
			}
		}
	}

	patched, err := json.MarshalIndent(proj, "", "  ")
	if err != nil {
		t.Fatalf("marshal patched project.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.json"), patched, 0o644); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	// Capture stdout to verify explain output.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	// Reset relevant flags before invocation.
	for _, name := range []string{"explain", "dry-run", "retrieval"} {
		if fl := generateCmd.Flags().Lookup(name); fl != nil {
			_ = fl.Value.Set("false")
			fl.Changed = false
		}
	}
	genExplain = false
	genDryRun = false
	genRetrieval = false

	rootCmd.SetArgs([]string{"generate", "-p", "evidence_mode_project", "--dry-run", "--explain"})
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = origStdout

	if execErr != nil {
		t.Fatalf("command failed: %v", execErr)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	for _, heading := range []string{
		"## Evidence Report",
		"### Project",
		"### Provider / Model",
		"### Prompt Statistics",
		"### Retrieval",
		"### Documents Included in Prompt",
	} {
		if !strings.Contains(out, heading) {
			t.Errorf("explain output missing heading %q\nfull output:\n%s", heading, out)
		}
	}

	for _, want := range []string{
		"evidence_mode_project",
		"quarterly-report.md",
		"risk-notes.txt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing expected content %q\nfull output:\n%s", want, out)
		}
	}
}