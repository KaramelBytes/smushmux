package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/project"
)

func TestBuildPromptIncludesDocsAndInstructions(t *testing.T) {
	tdir := t.TempDir()
	// Create sample docs
	p1 := filepath.Join(tdir, "a.txt")
	p2 := filepath.Join(tdir, "b.md")
	if err := os.WriteFile(p1, []byte("Alpha content."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("# Beta\n\nBeta content."), 0o644); err != nil {
		t.Fatal(err)
	}

	proj := project.NewProject("test", "", filepath.Join(tdir, "proj"))
	proj.SetInstructions("Summarize the documents")
	if err := proj.AddDocument(p1, "first"); err != nil {
		t.Fatalf("add doc1: %v", err)
	}
	if err := proj.AddDocument(p2, "second"); err != nil {
		t.Fatalf("add doc2: %v", err)
	}

	prompt, tokens, err := proj.BuildPrompt()
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if tokens <= 0 {
		t.Fatalf("expected tokens > 0")
	}
	if !strings.Contains(prompt, "[INSTRUCTIONS]") {
		t.Fatalf("missing instructions header")
	}
	if !strings.Contains(prompt, "Alpha content.") {
		t.Fatalf("missing doc1 content")
	}
	if !strings.Contains(prompt, "Beta content.") {
		t.Fatalf("missing doc2 content")
	}
	if !strings.Contains(prompt, "[TASK]") {
		t.Fatalf("missing task section")
	}
}
