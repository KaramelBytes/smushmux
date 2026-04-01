package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeBatch_AttachAndSuppressSamples(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", home)

	// Prepare two CSV files with the same basename in different directories
	d1 := filepath.Join(home, "d1")
	d2 := filepath.Join(home, "d2")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatalf("mkdir d1: %v", err)
	}
	if err := os.MkdirAll(d2, 0o755); err != nil {
		t.Fatalf("mkdir d2: %v", err)
	}
	csv := "col1,col2\nA,1\nB,2\nC,3\n"
	p1 := filepath.Join(d1, "metrics.csv")
	p2 := filepath.Join(d2, "metrics.csv")
	if err := os.WriteFile(p1, []byte(csv), 0o644); err != nil {
		t.Fatalf("write p1: %v", err)
	}
	if err := os.WriteFile(p2, []byte(csv), 0o644); err != nil {
		t.Fatalf("write p2: %v", err)
	}

	// Init a project
	runCmd(t, "init", "batchp", "-d", "batch project")

	// Analyze both with project attachment and disable sample tables
	runCmd(t, "analyze-batch", filepath.Join(home, "d*", "metrics.csv"), "-p", "batchp", "--sample-rows-project", "0")

	// Verify files written under dataset_summaries with collision suffix
	projDir, err := resolveProjectDirByName("batchp")
	if err != nil {
		t.Fatalf("resolve project: %v", err)
	}
	dsDir := filepath.Join(projDir, "dataset_summaries")
	b1 := filepath.Join(dsDir, "metrics.summary.md")
	b2 := filepath.Join(dsDir, "metrics__2.summary.md")
	if _, err := os.Stat(b1); err != nil {
		t.Fatalf("missing first summary: %v", err)
	}
	if _, err := os.Stat(b2); err != nil {
		t.Fatalf("missing second summary: %v", err)
	}

	// Assert sample rows are suppressed (no HEAD AND SAMPLE ROWS section)
	body1, err := os.ReadFile(b1)
	if err != nil {
		t.Fatalf("read b1: %v", err)
	}
	if strings.Contains(string(body1), "[HEAD AND SAMPLE ROWS]") {
		t.Fatalf("expected no sample rows in %s", b1)
	}
	body2, err := os.ReadFile(b2)
	if err != nil {
		t.Fatalf("read b2: %v", err)
	}
	if strings.Contains(string(body2), "[HEAD AND SAMPLE ROWS]") {
		t.Fatalf("expected no sample rows in %s", b2)
	}
}
