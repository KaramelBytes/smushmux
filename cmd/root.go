package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Global flags (wired later to config/viper)
	cfgFile string
	debug   bool
	// Retry/HTTP flags (override config if set)
	flagHTTPTimeoutSec   int
	flagRetryMaxAttempts int
	flagRetryBaseDelayMs int
	flagRetryMaxDelayMs  int

	// Loaded configuration
	cfg *cfgpkg.Global
)

var rootCmd = &cobra.Command{
	Use:   "smushmux",
	Short: "SmushMux CLI: merge multiple docs into one AI-ready context",
	Long:  `SmushMux is a CLI tool that combines multiple documents into a unified context and sends them to AI models via OpenRouter for analysis and generation.`,
}

// Execute is the entry point called by main.main()
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "✗ Error:", err)
		os.Exit(1)
	}
}

func init() {
	// Initialize configuration before executing commands
	cobra.OnInitialize(loadConfig)

	// Persistent global flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.smushmux/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().IntVar(&flagHTTPTimeoutSec, "http-timeout", 0, "HTTP client timeout in seconds (overrides config)")
	rootCmd.PersistentFlags().IntVar(&flagRetryMaxAttempts, "retry-max", 0, "max retry attempts on 429/5xx (overrides config)")
	rootCmd.PersistentFlags().IntVar(&flagRetryBaseDelayMs, "retry-base-ms", 0, "base retry backoff in ms (overrides config)")
	rootCmd.PersistentFlags().IntVar(&flagRetryMaxDelayMs, "retry-max-ms", 0, "max retry backoff cap in ms (overrides config)")
}

func loadConfig() {
	c, err := cfgpkg.Load(cfgFile)
	if err != nil {
		// Non-fatal: allow running commands that don't need config
		fmt.Fprintf(os.Stderr, "⚠ Warning: failed to load config: %v\n", err)
		if cfg == nil {
			cfg = &cfgpkg.Global{}
		}
		return
	}
	cfg = c

	// Apply CLI overrides if provided
	f := rootCmd.PersistentFlags()
	if f.Changed("http-timeout") && flagHTTPTimeoutSec > 0 {
		cfg.HTTPTimeoutSec = flagHTTPTimeoutSec
	}
	if f.Changed("retry-max") && flagRetryMaxAttempts > 0 {
		cfg.RetryMaxAttempts = flagRetryMaxAttempts
	}
	if f.Changed("retry-base-ms") && flagRetryBaseDelayMs > 0 {
		cfg.RetryBaseDelayMs = flagRetryBaseDelayMs
	}
	if f.Changed("retry-max-ms") && flagRetryMaxDelayMs > 0 {
		cfg.RetryMaxDelayMs = flagRetryMaxDelayMs
	}

	// Optional: auto-sync model catalog at startup
	if cfg.ModelsAutoSync {
		url := cfg.ModelsCatalogURL
		if url == "" && cfg.ModelsProvider != "" {
			// minimal provider mapping (keep in sync with models fetch presets)
			switch cfg.ModelsProvider {
			case "openrouter":
				url = "https://raw.githubusercontent.com/KaramelBytes/smushmux/main/docs/openrouter-models.json"
			}
		}
		if url != "" {
			if err := fetchAndApplyCatalog(url, cfg.ModelsMerge); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ Warning: models auto-sync failed: %v\n", err)
			}
		}
	}
}

// fetchAndApplyCatalog downloads a JSON catalog and applies it in-memory.
func fetchAndApplyCatalog(url string, merge bool) error {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("fetch: unexpected status %s: %s", resp.Status, string(b))
	}
	dec := json.NewDecoder(resp.Body)
	var m map[string]ai.ModelInfo
	if err := dec.Decode(&m); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if merge {
		ai.MergeCatalog(m)
	} else {
		ai.OverrideCatalog(m)
	}
	return nil
}
