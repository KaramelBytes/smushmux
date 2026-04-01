package utils_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/utils"
)

func TestEnsureProjectDirCreatesNestedPath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "c")

	if err := utils.EnsureProjectDir(target); err != nil {
		t.Fatalf("ensure project dir: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file")
	}
}

func TestSafeWriteFileWritesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")

	if err := utils.SafeWriteFile(path, []byte("first")); err != nil {
		t.Fatalf("first safe write: %v", err)
	}
	if err := utils.SafeWriteFile(path, []byte("second")); err != nil {
		t.Fatalf("second safe write: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(b) != "second" {
		t.Fatalf("unexpected file content: got %q", string(b))
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file should not remain after atomic write")
	}
}

func TestPrettyJSON(t *testing.T) {
	b, err := utils.PrettyJSON(map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("pretty json success case: %v", err)
	}
	if !strings.Contains(string(b), "\n") {
		t.Fatalf("expected indented JSON with newline")
	}

	if _, err := utils.PrettyJSON(make(chan int)); err == nil {
		t.Fatalf("expected marshal error for unsupported type")
	}
}

func TestFindProjectRootFromDirAndFile(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "project.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	gotFromDir, err := utils.FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("find project root from dir: %v", err)
	}
	if gotFromDir != root {
		t.Fatalf("root mismatch from dir: got %q want %q", gotFromDir, root)
	}

	filePath := filepath.Join(nested, "notes.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	gotFromFile, err := utils.FindProjectRoot(filePath)
	if err != nil {
		t.Fatalf("find project root from file: %v", err)
	}
	if gotFromFile != root {
		t.Fatalf("root mismatch from file: got %q want %q", gotFromFile, root)
	}
}

func TestFindProjectRootNotFound(t *testing.T) {
	start := t.TempDir()
	if _, err := utils.FindProjectRoot(start); err == nil {
		t.Fatalf("expected error when project root is not found")
	}
}
