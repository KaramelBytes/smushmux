package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/config"
)

func TestLoadDefaultsWithoutConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	if cfg.DefaultModel != "openai/gpt-4o-mini" {
		t.Fatalf("default_model mismatch: got %q", cfg.DefaultModel)
	}
	if cfg.DefaultProvider != "openrouter" {
		t.Fatalf("default_provider mismatch: got %q", cfg.DefaultProvider)
	}
	if cfg.RetrievalTopK != 6 {
		t.Fatalf("retrieval_top_k mismatch: got %d", cfg.RetrievalTopK)
	}
	wantProjectsDir := filepath.Join(home, ".smushmux", "projects")
	if cfg.ProjectsDir != wantProjectsDir {
		t.Fatalf("projects_dir mismatch: got %q want %q", cfg.ProjectsDir, wantProjectsDir)
	}
}

func TestLoadConfigFileAndEnvPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, "config.yaml")
	content := []byte("default_model: from_file\nretrieval_top_k: 4\n")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("SMUSHMUX_DEFAULT_MODEL", "from_env")
	t.Setenv("SMUSHMUX_RETRIEVAL_TOP_K", "9")

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config with env: %v", err)
	}

	if cfg.DefaultModel != "from_env" {
		t.Fatalf("env should override file for default_model: got %q", cfg.DefaultModel)
	}
	if cfg.RetrievalTopK != 9 {
		t.Fatalf("env should override file for retrieval_top_k: got %d", cfg.RetrievalTopK)
	}
}

func TestSaveExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")

	in := &config.Global{DefaultModel: "test/model", DefaultProvider: "openrouter", MaxTokens: 512}
	if err := config.Save(in, path); err != nil {
		t.Fatalf("save explicit path: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at explicit path: %v", err)
	}

	out, err := config.Load(path)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if out.DefaultModel != in.DefaultModel {
		t.Fatalf("default_model mismatch after round trip: got %q want %q", out.DefaultModel, in.DefaultModel)
	}
}

func TestSaveDefaultPathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	in := &config.Global{DefaultModel: "saved/default"}
	if err := config.Save(in, ""); err != nil {
		t.Fatalf("save default path: %v", err)
	}

	path := filepath.Join(home, ".smushmux", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected default config path to exist: %v", err)
	}
}
