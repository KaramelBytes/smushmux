package cmd

import (
	"fmt"
	"strconv"

	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
		Short: "View or set SmushMux configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Println("No config loaded")
			return nil
		}
		fmt.Printf("api_key: %s\n", mask(cfg.APIKey))
		fmt.Printf("default_model: %s\n", cfg.DefaultModel)
		if cfg.DefaultProvider != "" {
			fmt.Printf("default_provider: %s\n", cfg.DefaultProvider)
		}
		if cfg.EmbeddingModel != "" {
			fmt.Printf("embedding_model: %s\n", cfg.EmbeddingModel)
		}
		if cfg.EmbeddingProvider != "" {
			fmt.Printf("embedding_provider: %s\n", cfg.EmbeddingProvider)
		}
		if cfg.RetrievalTopK > 0 {
			fmt.Printf("retrieval_top_k: %d\n", cfg.RetrievalTopK)
		}
		if cfg.RetrievalMinScore >= 0 {
			fmt.Printf("retrieval_min_score: %.3f\n", cfg.RetrievalMinScore)
		}
		fmt.Printf("max_tokens: %d\n", cfg.MaxTokens)
		fmt.Printf("temperature: %.3f\n", cfg.Temperature)
		fmt.Printf("projects_dir: %s\n", cfg.ProjectsDir)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value and save to disk",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, val := args[0], args[1]
		if cfg == nil {
			c, err := cfgpkg.Load(cfgFile)
			if err != nil {
				return err
			}
			cfg = c
		}
		switch key {
		case "api_key":
			cfg.APIKey = val
		case "default_model":
			cfg.DefaultModel = val
		case "default_provider":
			switch val {
			case "openrouter", "OpenRouter", "OPENROUTER":
				cfg.DefaultProvider = "openrouter"
			case "ollama", "local", "Ollama", "LOCAL":
				cfg.DefaultProvider = "ollama"
			default:
				return fmt.Errorf("invalid default_provider: %s (use openrouter or ollama)", val)
			}
		case "embedding_model":
			cfg.EmbeddingModel = val
		case "embedding_provider":
			switch val {
			case "openrouter", "OpenRouter", "OPENROUTER":
				cfg.EmbeddingProvider = "openrouter"
			case "ollama", "Ollama", "LOCAL", "local":
				cfg.EmbeddingProvider = "ollama"
			default:
				return fmt.Errorf("invalid embedding_provider: %s (use openrouter or ollama)", val)
			}
		case "retrieval_top_k":
			i, err := strconv.Atoi(val)
			if err != nil || i < 0 {
				return fmt.Errorf("invalid int for retrieval_top_k: %v", val)
			}
			cfg.RetrievalTopK = i
		case "retrieval_min_score":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil || f < 0 {
				return fmt.Errorf("invalid float for retrieval_min_score: %v", val)
			}
			cfg.RetrievalMinScore = f
		case "max_tokens":
			i, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("invalid int for max_tokens: %w", err)
			}
			cfg.MaxTokens = i
		case "temperature":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("invalid float for temperature: %w", err)
			}
			cfg.Temperature = f
		case "projects_dir":
			cfg.ProjectsDir = val
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
		if err := cfgpkg.Save(cfg, cfgFile); err != nil {
			return err
		}
		fmt.Println("Saved config")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}

func mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "****" + s[len(s)-3:]
}
