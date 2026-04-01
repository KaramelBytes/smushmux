package ai

// PresetCatalog returns a built-in curated catalog for a known provider.
// The catalog can be merged or used to replace the in-memory catalog.
func PresetCatalog(provider string) (map[string]ModelInfo, bool) {
	switch provider {
	case "openrouter":
		// Curated minimal set; values illustrative and aligned with models.go defaults
		return map[string]ModelInfo{
			"anthropic/claude-3.5-sonnet": {
				Name:          "anthropic/claude-3.5-sonnet",
				ContextTokens: 200000,
				InputPerK:     0.003,
				OutputPerK:    0.015,
			},
			"openai/gpt-4o-mini": {
				Name:          "openai/gpt-4o-mini",
				ContextTokens: 128000,
				InputPerK:     0.0006,
				OutputPerK:    0.0024,
			},
			"openai/gpt-4o": {
				Name:          "openai/gpt-4o",
				ContextTokens: 128000,
				InputPerK:     0.005,
				OutputPerK:    0.015,
			},
			"openai/gpt-4.1-mini": {
				Name:          "openai/gpt-4.1-mini",
				ContextTokens: 128000,
				InputPerK:     0.0005,
				OutputPerK:    0.0015,
			},
			"deepseek/deepseek-r1:free": {
				Name:          "deepseek/deepseek-r1:free",
				ContextTokens: 128000,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
		}, true
	case "openai":
		// Placeholder curated list referencing OpenAI naming via OpenRouter
		return map[string]ModelInfo{
			"openai/gpt-4o": {
				Name:          "openai/gpt-4o",
				ContextTokens: 128000,
				InputPerK:     0.005,
				OutputPerK:    0.015,
			},
			"openai/gpt-4o-mini": {
				Name:          "openai/gpt-4o-mini",
				ContextTokens: 128000,
				InputPerK:     0.0006,
				OutputPerK:    0.0024,
			},
		}, true
	case "anthropic":
		return map[string]ModelInfo{
			"anthropic/claude-3.5-sonnet": {
				Name:          "anthropic/claude-3.5-sonnet",
				ContextTokens: 200000,
				InputPerK:     0.003,
				OutputPerK:    0.015,
			},
		}, true
	case "google", "gemini":
		return map[string]ModelInfo{
			"google/gemini-1.5-flash": {
				Name:          "google/gemini-1.5-flash",
				ContextTokens: 1000000, // generous context for flash variants
				InputPerK:     0.0002,
				OutputPerK:    0.0008,
			},
			"google/gemini-1.5-pro": {
				Name:          "google/gemini-1.5-pro",
				ContextTokens: 1000000,
				InputPerK:     0.00125,
				OutputPerK:    0.005,
			},
		}, true
	case "meta", "llama":
		return map[string]ModelInfo{
			"meta-llama/llama-3.1-8b-instruct": {
				Name:          "meta-llama/llama-3.1-8b-instruct",
				ContextTokens: 131072,
				InputPerK:     0.0000, // many deployments are free/local; treat as ~0 for hints
				OutputPerK:    0.0000,
			},
			"meta-llama/llama-3.1-70b-instruct": {
				Name:          "meta-llama/llama-3.1-70b-instruct",
				ContextTokens: 131072,
				InputPerK:     0.0000,
				OutputPerK:    0.0000,
			},
		}, true
	case "ollama", "local":
		// Local-friendly defaults that commonly exist in Ollama registries
		return map[string]ModelInfo{
			"llama3:latest": {
				Name:          "llama3:latest",
				ContextTokens: 8192,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"llama3.1:8b-instruct": {
				Name:          "llama3.1:8b-instruct",
				ContextTokens: 8192,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"llama3.1:70b-instruct": {
				Name:          "llama3.1:70b-instruct",
				ContextTokens: 8192,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"mistral-nemo:latest": {
				Name:          "mistral-nemo:latest",
				ContextTokens: 8192,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"mistral:7b-instruct": {
				Name:          "mistral:7b-instruct",
				ContextTokens: 8192,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"phi3:mini-4k-instruct": {
				Name:          "phi3:mini-4k-instruct",
				ContextTokens: 4096,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
			"phi3:mini-128k-instruct": {
				Name:          "phi3:mini-128k-instruct",
				ContextTokens: 128000,
				InputPerK:     0.0,
				OutputPerK:    0.0,
			},
		}, true
	default:
		return nil, false
	}
}

// RecommendModel returns a recommended model name for a given tier and provider.
// If provider is empty, defaults to "openrouter". Tiers: cheap|balanced|high-context.
func RecommendModel(provider, tier string) (string, bool) {
	if provider == "" {
		provider = "openrouter"
	}
	switch tier {
	case "cheap":
		switch provider {
		case "openrouter":
			return "deepseek/deepseek-r1:free", true
		case "openai":
			return "openai/gpt-4o-mini", true
		case "anthropic":
			return "anthropic/claude-3-haiku", true
		case "google", "gemini":
			return "google/gemini-1.5-flash", true
		case "meta", "llama":
			return "meta-llama/llama-3.1-8b-instruct", true
		}
	case "balanced":
		switch provider {
		case "openrouter", "openai":
			return "openai/gpt-4o", true
		case "anthropic":
			return "anthropic/claude-3.5-sonnet", true
		case "google", "gemini":
			return "google/gemini-1.5-pro", true
		case "meta", "llama":
			return "meta-llama/llama-3.1-70b-instruct", true
		}
	case "high-context":
		switch provider {
		case "openrouter", "anthropic":
			return "anthropic/claude-3.5-sonnet", true // ~200k context
		case "openai":
			return "openai/gpt-4o", true // 128k context
		case "google", "gemini":
			return "google/gemini-1.5-pro", true // very large context
		case "meta", "llama":
			return "meta-llama/llama-3.1-70b-instruct", true
		}
	}
	return "", false
}
