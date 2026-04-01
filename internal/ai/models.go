package ai

import (
	"encoding/json"
	"os"
)

// Model metadata and simple pricing helpers for UX warnings.
// Prices are illustrative and should be verified against OpenRouter docs.

type ModelInfo struct {
	Name          string
	ContextTokens int     // approximate context window
	InputPerK     float64 // USD per 1K input tokens
	OutputPerK    float64 // USD per 1K output tokens
}

var models = map[string]ModelInfo{
	// Deepseek free tier via OpenRouter
	"deepseek/deepseek-r1:free": {
		Name:          "deepseek/deepseek-r1:free",
		ContextTokens: 128000,
		InputPerK:     0.0,
		OutputPerK:    0.0,
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
	"anthropic/claude-3.5-sonnet": {
		Name:          "anthropic/claude-3.5-sonnet",
		ContextTokens: 200000,
		InputPerK:     0.003,
		OutputPerK:    0.015,
	},
	"anthropic/claude-3-haiku": {
		Name:          "anthropic/claude-3-haiku",
		ContextTokens: 200000,
		InputPerK:     0.00025,
		OutputPerK:    0.00125,
	},
	// Google Gemini
	"google/gemini-1.5-flash": {
		Name:          "google/gemini-1.5-flash",
		ContextTokens: 1000000,
		InputPerK:     0.0002,
		OutputPerK:    0.0008,
	},
	"google/gemini-1.5-pro": {
		Name:          "google/gemini-1.5-pro",
		ContextTokens: 1000000,
		InputPerK:     0.00125,
		OutputPerK:    0.005,
	},
	// Meta Llama 3.1 (common OpenRouter names)
	"meta-llama/llama-3.1-8b-instruct": {
		Name:          "meta-llama/llama-3.1-8b-instruct",
		ContextTokens: 131072,
		InputPerK:     0.0,
		OutputPerK:    0.0,
	},
	"meta-llama/llama-3.1-70b-instruct": {
		Name:          "meta-llama/llama-3.1-70b-instruct",
		ContextTokens: 131072,
		InputPerK:     0.0,
		OutputPerK:    0.0,
	},
	// Common local (Ollama) tags
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
}

// LookupModel returns ModelInfo and ok flag.
func LookupModel(name string) (ModelInfo, bool) {
	mi, ok := models[name]
	return mi, ok
}

// EstimateCostUSD estimates total cost in USD for given tokens using model pricing.
// If the model is unknown, returns 0 and ok=false.
func EstimateCostUSD(model string, promptTokens, completionTokens int) (float64, bool) {
	mi, ok := LookupModel(model)
	if !ok {
		return 0, false
	}
	inCost := (float64(promptTokens) / 1000.0) * mi.InputPerK
	outCost := (float64(completionTokens) / 1000.0) * mi.OutputPerK
	return inCost + outCost, true
}

// ---- Sync/override helpers ----

// LoadCatalogFromJSON loads a JSON object map[string]ModelInfo from a file path.
// Example JSON entry:
// Example JSON entry:
// { "openai/gpt-4o-mini": {"Name":"openai/gpt-4o-mini","ContextTokens":128000,"InputPerK":0.0006,"OutputPerK":0.0024} }
func LoadCatalogFromJSON(path string) (map[string]ModelInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var m map[string]ModelInfo
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// OverrideCatalog replaces the in-memory catalog entirely.
func OverrideCatalog(m map[string]ModelInfo) {
	if m == nil {
		return
	}
	models = m
}

// MergeCatalog merges/overrides entries in the in-memory catalog.
func MergeCatalog(m map[string]ModelInfo) {
	if m == nil {
		return
	}
	for k, v := range m {
		models[k] = v
	}
}

// Catalog returns a shallow copy of the current model catalog.
func Catalog() map[string]ModelInfo {
	out := make(map[string]ModelInfo, len(models))
	for k, v := range models {
		out[k] = v
	}
	return out
}
