package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/ai"
)

func preserveRootGlobals(t *testing.T) {
	t.Helper()
	origCfg := cfg
	origCfgFile := cfgFile
	t.Cleanup(func() {
		cfg = origCfg
		cfgFile = origCfgFile
	})
}

func resetPersistentFlag(t *testing.T, name, value string) {
	t.Helper()
	f := rootCmd.PersistentFlags().Lookup(name)
	if f == nil {
		t.Fatalf("missing persistent flag: %s", name)
	}
	if err := f.Value.Set(value); err != nil {
		t.Fatalf("reset flag %s: %v", name, err)
	}
	f.Changed = false
}

func TestLoadConfigAppliesFlagOverrides(t *testing.T) {
	preserveRootGlobals(t)

	dir := t.TempDir()
	cfgFile = filepath.Join(dir, "config.yaml")
	yml := []byte("http_timeout_sec: 10\nretry_max_attempts: 2\nretry_base_delay_ms: 100\nretry_max_delay_ms: 200\n")
	if err := os.WriteFile(cfgFile, yml, 0o644); err != nil {
		t.Fatalf("write cfg file: %v", err)
	}

	t.Cleanup(func() {
		resetPersistentFlag(t, "http-timeout", "0")
		resetPersistentFlag(t, "retry-max", "0")
		resetPersistentFlag(t, "retry-base-ms", "0")
		resetPersistentFlag(t, "retry-max-ms", "0")
		flagHTTPTimeoutSec = 0
		flagRetryMaxAttempts = 0
		flagRetryBaseDelayMs = 0
		flagRetryMaxDelayMs = 0
	})

	if err := rootCmd.PersistentFlags().Set("http-timeout", "77"); err != nil {
		t.Fatalf("set http-timeout: %v", err)
	}
	if err := rootCmd.PersistentFlags().Set("retry-max", "9"); err != nil {
		t.Fatalf("set retry-max: %v", err)
	}
	if err := rootCmd.PersistentFlags().Set("retry-base-ms", "333"); err != nil {
		t.Fatalf("set retry-base-ms: %v", err)
	}
	if err := rootCmd.PersistentFlags().Set("retry-max-ms", "444"); err != nil {
		t.Fatalf("set retry-max-ms: %v", err)
	}

	loadConfig()
	if cfg == nil {
		t.Fatalf("expected cfg to be loaded")
	}
	if cfg.HTTPTimeoutSec != 77 || cfg.RetryMaxAttempts != 9 || cfg.RetryBaseDelayMs != 333 || cfg.RetryMaxDelayMs != 444 {
		t.Fatalf("override mismatch: got timeout=%d retry-max=%d base=%d max=%d", cfg.HTTPTimeoutSec, cfg.RetryMaxAttempts, cfg.RetryBaseDelayMs, cfg.RetryMaxDelayMs)
	}
}

func TestFetchAndApplyCatalogSuccessAndStatusError(t *testing.T) {
	orig := ai.Catalog()
	t.Cleanup(func() { ai.OverrideCatalog(orig) })

	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"unit/test-model":{"Name":"unit/test-model","ContextTokens":1234,"InputPerK":0.1,"OutputPerK":0.2}}`)
	}))
	defer tsOK.Close()

	if err := fetchAndApplyCatalog(tsOK.URL, false); err != nil {
		t.Fatalf("fetchAndApplyCatalog success: %v", err)
	}
	cat := ai.Catalog()
	if _, ok := cat["unit/test-model"]; !ok {
		t.Fatalf("expected fetched model in catalog")
	}

	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))
	defer tsErr.Close()

	err := fetchAndApplyCatalog(tsErr.URL, true)
	if err == nil {
		t.Fatalf("expected non-2xx status error")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("unexpected error text: %v", err)
	}
}
