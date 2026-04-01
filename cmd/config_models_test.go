package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
)

func preserveCmdGlobals(t *testing.T) {
	t.Helper()
	origCfg := cfg
	origCfgFile := cfgFile
	origSyncPath := syncPath
	origSyncMerge := syncMerge
	origFetchURL := fetchURL
	origFetchOutput := fetchOutput
	origFetchMerge := fetchMerge
	origFetchProvider := fetchProvider
	t.Cleanup(func() {
		cfg = origCfg
		cfgFile = origCfgFile
		syncPath = origSyncPath
		syncMerge = origSyncMerge
		fetchURL = origFetchURL
		fetchOutput = origFetchOutput
		fetchMerge = origFetchMerge
		fetchProvider = origFetchProvider
	})
}

func TestMask(t *testing.T) {
	if got := mask(""); got != "" {
		t.Fatalf("mask empty: got %q", got)
	}
	if got := mask("abc"); got != "******" {
		t.Fatalf("mask short: got %q", got)
	}
	if got := mask("abcdefghi"); got != "abc****ghi" {
		t.Fatalf("mask long: got %q", got)
	}
}

func TestConfigSetValidationAndSave(t *testing.T) {
	preserveCmdGlobals(t)

	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "config.yaml")
	cfg = &cfgpkg.Global{}

	if err := configSetCmd.RunE(configSetCmd, []string{"default_provider", "bad"}); err == nil {
		t.Fatalf("expected invalid default_provider error")
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"retrieval_top_k", "-1"}); err == nil {
		t.Fatalf("expected invalid retrieval_top_k error")
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"temperature", "not-a-number"}); err == nil {
		t.Fatalf("expected invalid temperature error")
	}

	if err := configSetCmd.RunE(configSetCmd, []string{"default_provider", "LOCAL"}); err != nil {
		t.Fatalf("set default_provider: %v", err)
	}
	if cfg.DefaultProvider != "ollama" {
		t.Fatalf("expected normalized provider 'ollama', got %q", cfg.DefaultProvider)
	}

	if err := configSetCmd.RunE(configSetCmd, []string{"max_tokens", "2048"}); err != nil {
		t.Fatalf("set max_tokens: %v", err)
	}
	if cfg.MaxTokens != 2048 {
		t.Fatalf("expected max_tokens=2048, got %d", cfg.MaxTokens)
	}

	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected config to be saved: %v", err)
	}
}

func TestProviderURL(t *testing.T) {
	preserveCmdGlobals(t)

	t.Setenv("SMUSHMUX_OPENROUTER_CATALOG_URL", "https://example.test/openrouter.json")
	if got := providerURL("openrouter"); got != "https://example.test/openrouter.json" {
		t.Fatalf("openrouter env override mismatch: got %q", got)
	}

	t.Setenv("SMUSHMUX_OPENROUTER_CATALOG_URL", "")
	if got := providerURL("openrouter"); !strings.Contains(got, "openrouter-models.json") {
		t.Fatalf("openrouter default url mismatch: got %q", got)
	}

	if got := providerURL("unknown"); got != "" {
		t.Fatalf("unknown provider should return empty URL, got %q", got)
	}
}

func TestModelsFetchBuiltInPresetNoNetwork(t *testing.T) {
	preserveCmdGlobals(t)

	original := ai.Catalog()
	t.Cleanup(func() { ai.OverrideCatalog(original) })

	dir := t.TempDir()
	out := filepath.Join(dir, "catalog.json")

	fetchURL = ""
	fetchProvider = "openai"
	fetchMerge = false
	fetchOutput = out

	if err := modelsFetchCmd.RunE(modelsFetchCmd, nil); err != nil {
		t.Fatalf("models fetch preset: %v", err)
	}

	cat := ai.Catalog()
	if _, ok := cat["openai/gpt-4o-mini"]; !ok {
		t.Fatalf("expected preset model in catalog after fetch preset")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected preset output file: %v", err)
	}
}

func TestModelsSyncRequiresFile(t *testing.T) {
	preserveCmdGlobals(t)

	syncPath = ""
	syncMerge = false
	if err := modelsSyncCmd.RunE(modelsSyncCmd, nil); err == nil {
		t.Fatalf("expected --file required error")
	}
}
