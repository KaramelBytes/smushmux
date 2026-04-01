package ai

import "time"

// RuntimeFactory builds a Runtime from the generic config below.
type RuntimeFactory func(RuntimeConfig) Runtime

// RuntimeConfig carries common knobs used by runtimes.
type RuntimeConfig struct {
	// Common
	HTTPTimeout time.Duration
	RetryMax    int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	// OpenRouter
	APIKey string
	// Ollama
	Host string
}

var registry = map[string]RuntimeFactory{}

// RegisterRuntime registers a provider name with its factory.
func RegisterRuntime(name string, f RuntimeFactory) { registry[name] = f }

// GetRuntime creates a Runtime for the given provider if registered.
func GetRuntime(name string, cfg RuntimeConfig) (Runtime, bool) {
	if f, ok := registry[name]; ok {
		return f(cfg), true
	}
	return nil, false
}

// init registers built-in runtimes.
func init() {
	RegisterRuntime(ProviderOpenRouter, func(c RuntimeConfig) Runtime {
		if c.HTTPTimeout <= 0 {
			c.HTTPTimeout = 60 * time.Second
		}
		if c.RetryMax <= 0 {
			c.RetryMax = 3
		}
		if c.BaseDelay <= 0 {
			c.BaseDelay = 500 * time.Millisecond
		}
		if c.MaxDelay <= 0 {
			c.MaxDelay = 4 * time.Second
		}
		return NewClient(c.APIKey, c.HTTPTimeout, c.RetryMax, c.BaseDelay, c.MaxDelay)
	})
	RegisterRuntime(ProviderOllama, func(c RuntimeConfig) Runtime {
		if c.HTTPTimeout <= 0 {
			c.HTTPTimeout = 60 * time.Second
		}
		if c.RetryMax <= 0 {
			c.RetryMax = 2
		}
		if c.BaseDelay <= 0 {
			c.BaseDelay = 200 * time.Millisecond
		}
		if c.MaxDelay <= 0 {
			c.MaxDelay = 1 * time.Second
		}
		return NewOllamaClient(c.Host, c.HTTPTimeout, c.RetryMax, c.BaseDelay, c.MaxDelay)
	})
}
