package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/KaramelBytes/smushmux/internal/ai"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage or inspect model catalog and pricing",
	Example: `  smushmux models show
	smushmux models sync --file ./models.json
	smushmux models sync --file ./models.json --merge
	smushmux models fetch --url https://example.com/models.json
	smushmux models fetch --provider openrouter --merge --output models.json`,
}

var modelsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current model catalog",
	RunE: func(cmd *cobra.Command, args []string) error {
		cat := ai.Catalog()
		// pretty-print deterministic order
		keys := make([]string, 0, len(cat))
		for k := range cat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		m := make(map[string]ai.ModelInfo, len(keys))
		for _, k := range keys {
			m[k] = cat[k]
		}
		return enc.Encode(m)
	},
}

var (
	syncPath  string
	syncMerge bool
)

var modelsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Load model catalog/pricing from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if syncPath == "" {
			return fmt.Errorf("--file is required")
		}
		m, err := ai.LoadCatalogFromJSON(syncPath)
		if err != nil {
			return fmt.Errorf("load catalog: %w", err)
		}
		if syncMerge {
			ai.MergeCatalog(m)
			fmt.Println("Merged model catalog from file")
		} else {
			ai.OverrideCatalog(m)
			fmt.Println("Replaced model catalog from file")
		}
		return nil
	},
}

// providerURL returns a preset URL for a known provider. Empty string if unknown.
func providerURL(name string) string {
	// Allow environment overrides for provider catalog URLs
	// SMUSHMUX_OPENROUTER_CATALOG_URL, SMUSHMUX_OPENAI_CATALOG_URL, SMUSHMUX_ANTHROPIC_CATALOG_URL
	switch name {
	case "openrouter":
		if v := os.Getenv("SMUSHMUX_OPENROUTER_CATALOG_URL"); v != "" {
			return v
		}
		// default maintained endpoint
		return "https://raw.githubusercontent.com/KaramelBytes/smushmux/main/docs/openrouter-models.json"
	case "openai":
		if v := os.Getenv("SMUSHMUX_OPENAI_CATALOG_URL"); v != "" {
			return v
		}
		return ""
	case "anthropic":
		if v := os.Getenv("SMUSHMUX_ANTHROPIC_CATALOG_URL"); v != "" {
			return v
		}
		return ""
	default:
		return ""
	}
}

var (
	fetchURL      string
	fetchOutput   string
	fetchMerge    bool
	fetchProvider string
)

var modelsFetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch model catalog/pricing JSON from a URL and apply it",
	RunE: func(cmd *cobra.Command, args []string) error {
		if fetchURL == "" && fetchProvider != "" {
			if u := providerURL(fetchProvider); u != "" {
				fetchURL = u
			}
		}
		// If no URL, but a known provider preset exists, apply it locally without network.
		if fetchURL == "" && fetchProvider != "" {
			if preset, ok := ai.PresetCatalog(fetchProvider); ok {
				if fetchMerge {
					ai.MergeCatalog(preset)
					fmt.Printf("Merged built-in '%s' preset into in-memory catalog\n", fetchProvider)
				} else {
					ai.OverrideCatalog(preset)
					fmt.Printf("Replaced in-memory catalog with built-in '%s' preset\n", fetchProvider)
				}
				// Optionally write to file
				if fetchOutput != "" {
					data, err := json.MarshalIndent(preset, "", "  ")
					if err != nil {
						return fmt.Errorf("marshal: %w", err)
					}
					if err := os.WriteFile(fetchOutput, data, 0o644); err != nil {
						return fmt.Errorf("write file: %w", err)
					}
					fmt.Printf("Saved preset catalog to %s\n", fetchOutput)
				}
				return nil
			}
		}
		if fetchURL == "" {
			return fmt.Errorf("--url is required (or specify --provider with a known preset)")
		}
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Get(fetchURL)
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
		// Optionally write to file
		if fetchOutput != "" {
			data, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}
			if err := os.WriteFile(fetchOutput, data, 0o644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
			fmt.Printf("Saved catalog to %s\n", fetchOutput)
		}
		if fetchMerge {
			ai.MergeCatalog(m)
			fmt.Println("Merged fetched catalog into in-memory catalog")
		} else {
			ai.OverrideCatalog(m)
			fmt.Println("Replaced in-memory catalog with fetched catalog")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsShowCmd)
	modelsCmd.AddCommand(modelsSyncCmd)
	modelsCmd.AddCommand(modelsFetchCmd)

	modelsSyncCmd.Flags().StringVar(&syncPath, "file", "", "path to JSON catalog file")
	modelsSyncCmd.Flags().BoolVar(&syncMerge, "merge", false, "merge into existing catalog instead of replacing")

	modelsFetchCmd.Flags().StringVar(&fetchURL, "url", "", "URL to JSON catalog file")
	modelsFetchCmd.Flags().StringVar(&fetchOutput, "output", "", "optional path to save the fetched JSON")
	modelsFetchCmd.Flags().BoolVar(&fetchMerge, "merge", false, "merge into existing catalog instead of replacing")
	modelsFetchCmd.Flags().StringVar(&fetchProvider, "provider", "", "provider preset (e.g. 'openrouter') to resolve the catalog URL if --url is not set")
}
