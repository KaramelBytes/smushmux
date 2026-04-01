package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/parser"
)

func TestParseFileTXT(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	content := "hello world\nthis is txt"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := parser.ParseFile(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) == 0 || out[:5] != "hello" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestParseFileMD(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	content := "# Title\n\nBody here\n\n- list\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := parser.ParseFile(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) == 0 || out[:5] != "# Tit" {
		t.Fatalf("unexpected output: %q", out)
	}
}
