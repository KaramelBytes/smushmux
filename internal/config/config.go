package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Global configuration structure.
type Global struct {
	APIKey                   string   `mapstructure:"api_key" yaml:"api_key"`
	DefaultModel             string   `mapstructure:"default_model" yaml:"default_model"`
	DefaultProvider          string   `mapstructure:"default_provider" yaml:"default_provider"`
	EmbeddingModel           string   `mapstructure:"embedding_model" yaml:"embedding_model"`
	EmbeddingProvider        string   `mapstructure:"embedding_provider" yaml:"embedding_provider"`
	RetrievalTopK            int      `mapstructure:"retrieval_top_k" yaml:"retrieval_top_k"`
	RetrievalMinScore        float64  `mapstructure:"retrieval_min_score" yaml:"retrieval_min_score"`
	RetrievalInclude         []string `mapstructure:"retrieval_include" yaml:"retrieval_include"`
	RetrievalExclude         []string `mapstructure:"retrieval_exclude" yaml:"retrieval_exclude"`
	RetrievalMaxChunksPerDoc int      `mapstructure:"retrieval_max_chunks_per_doc" yaml:"retrieval_max_chunks_per_doc"`
	MaxTokens                int      `mapstructure:"max_tokens" yaml:"max_tokens"`
	Temperature              float64  `mapstructure:"temperature" yaml:"temperature"`
	ProjectsDir              string   `mapstructure:"projects_dir" yaml:"projects_dir"`
	// Models catalog auto-sync
	ModelsCatalogURL string `mapstructure:"models_catalog_url" yaml:"models_catalog_url"`
	ModelsAutoSync   bool   `mapstructure:"models_auto_sync" yaml:"models_auto_sync"`
	ModelsMerge      bool   `mapstructure:"models_merge" yaml:"models_merge"`
	ModelsProvider   string `mapstructure:"models_provider" yaml:"models_provider"`

	// HTTP/Retry configuration
	HTTPTimeoutSec   int `mapstructure:"http_timeout_sec" yaml:"http_timeout_sec"`
	RetryMaxAttempts int `mapstructure:"retry_max_attempts" yaml:"retry_max_attempts"`
	RetryBaseDelayMs int `mapstructure:"retry_base_delay_ms" yaml:"retry_base_delay_ms"`
	RetryMaxDelayMs  int `mapstructure:"retry_max_delay_ms" yaml:"retry_max_delay_ms"`

	// Local runtimes (Ollama)
	OllamaHost       string `mapstructure:"ollama_host" yaml:"ollama_host"`
	OllamaTimeoutSec int    `mapstructure:"ollama_timeout_sec" yaml:"ollama_timeout_sec"`
	MaxContextCap    int    `mapstructure:"max_context_cap" yaml:"max_context_cap"`
}

// Save writes the given configuration to the cfgFile path. If cfgFile is empty,
// it writes to ~/.smushmux/config.yaml, creating the directory if necessary.
func Save(c *Global, cfgFile string) error {
	var path string
	if cfgFile != "" {
		path = cfgFile
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home dir: %w", err)
		}
		dir := filepath.Join(home, ".smushmux")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir config dir: %w", err)
		}
		path = filepath.Join(dir, "config.yaml")
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Load loads configuration from file, env, and defaults.
// Precedence: flags (cfgFile) > env > config file > defaults.
func Load(cfgFile string) (*Global, error) {
	v := viper.New()
	// New prefix for SmushMux
	v.SetEnvPrefix("SMUSHMUX")
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("default_model", "openai/gpt-4o-mini")
	v.SetDefault("default_provider", "openrouter")
	v.SetDefault("embedding_model", "openai/text-embedding-3-small")
	v.SetDefault("embedding_provider", "openrouter")
	v.SetDefault("retrieval_top_k", 6)
	v.SetDefault("retrieval_min_score", 0.0)
	v.SetDefault("retrieval_include", []string{})
	v.SetDefault("retrieval_exclude", []string{})
	v.SetDefault("retrieval_max_chunks_per_doc", 0)
	v.SetDefault("max_tokens", 4096)
	v.SetDefault("temperature", 0.7)
	v.SetDefault("models_auto_sync", false)
	v.SetDefault("models_merge", true)
	v.SetDefault("models_provider", "")
	// HTTP/retry defaults
	v.SetDefault("http_timeout_sec", 60)
	v.SetDefault("retry_max_attempts", 3)
	v.SetDefault("retry_base_delay_ms", 500)
	v.SetDefault("retry_max_delay_ms", 4000)
	// Ollama defaults
	v.SetDefault("ollama_host", "http://127.0.0.1:11434")
	v.SetDefault("ollama_timeout_sec", 60)
	v.SetDefault("max_context_cap", 0) // 0 means use the provider's model preset

	// Config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		dir := filepath.Join(home, ".smushmux")
		_ = os.MkdirAll(dir, 0o755)
		v.AddConfigPath(dir)
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}
	// optional read
	_ = v.ReadInConfig()

	var c Global
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	// Resolve projects_dir default: ~/.smushmux/projects
	if c.ProjectsDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		c.ProjectsDir = filepath.Join(home, ".smushmux", "projects")
	}
	return &c, nil
}
