package ai

import "context"

// Runtime is a minimal interface implemented by AI backends/runtimes
// such as OpenRouter and local runtimes (e.g., Ollama).
// It aligns to the shared request/response types in this package.
type Runtime interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

// Provider identifiers used across the CLI for selection.
const (
	ProviderOpenRouter = "openrouter"
	ProviderOpenAI     = "openai"
	ProviderAnthropic  = "anthropic"
	ProviderGoogle     = "google"
	ProviderGemini     = "gemini"
	ProviderMeta       = "meta"
	ProviderLlama      = "llama"
	ProviderOllama     = "ollama"
	ProviderLocal      = "local"
)

// StreamRuntime is an optional extension that supports streaming output.
// Implementors should invoke onDelta with each partial content chunk.
type StreamRuntime interface {
	GenerateStream(ctx context.Context, req GenerateRequest, onDelta func(string)) error
}
