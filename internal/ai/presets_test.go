package ai

import "testing"

func TestPresetCatalogOpenRouter(t *testing.T) {
	m, ok := PresetCatalog("openrouter")
	if !ok || len(m) == 0 {
		t.Fatalf("expected openrouter preset to be available")
	}
	if _, exists := m["openai/gpt-4o-mini"]; !exists {
		t.Fatalf("expected gpt-4o-mini in openrouter preset")
	}
	if _, exists := m["deepseek/deepseek-r1:free"]; !exists {
		t.Fatalf("expected deepseek free model in openrouter preset")
	}
}

func TestRecommendModel(t *testing.T) {
	if name, ok := RecommendModel("openrouter", "cheap"); !ok || name != "deepseek/deepseek-r1:free" {
		t.Fatalf("unexpected recommendation for openrouter/cheap: %s", name)
	}
	if name, ok := RecommendModel("anthropic", "balanced"); !ok || name != "anthropic/claude-3.5-sonnet" {
		t.Fatalf("unexpected recommendation for anthropic/balanced: %s", name)
	}
	if name, ok := RecommendModel("google", "cheap"); !ok || name != "google/gemini-1.5-flash" {
		t.Fatalf("unexpected recommendation for google/cheap: %s", name)
	}
	if name, ok := RecommendModel("llama", "balanced"); !ok || name != "meta-llama/llama-3.1-70b-instruct" {
		t.Fatalf("unexpected recommendation for llama/balanced: %s", name)
	}
	if _, ok := RecommendModel("", "unknown"); ok {
		t.Fatalf("expected unknown tier to be false")
	}
}
