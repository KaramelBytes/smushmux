package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KaramelBytes/smushmux/internal/ai"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
	"github.com/KaramelBytes/smushmux/internal/utils"
)

type retrievalOptions struct {
	Enabled         bool
	Reindex         bool
	EmbedModel      string
	EmbedProvider   string
	TopK            int
	MinScore        float64
	OllamaHost      string
	MaxChunksPerDoc int // 0 = no per-doc cap
}

type retrievalDeps struct {
	newEmbedder func(ctx context.Context, provider, model string, cfg *cfgpkg.Global, opts retrievalOptions) (retrieval.Embedder, error)
	buildIndex  func(ctx context.Context, emb retrieval.Embedder, root string, docs map[string]struct{ Name, Content string }, opts retrieval.BuildOptions) (*retrieval.Index, error)
}

var defaultRetrievalDeps = retrievalDeps{
	newEmbedder: defaultNewEmbedder,
	buildIndex:  retrieval.BuildIndex,
}

func prepareRetrievedContext(ctx context.Context, p *project.Project, prompt string, baseTokens int, cfg *cfgpkg.Global, opts retrievalOptions, deps retrievalDeps) (string, int, []retrieval.ScoredRecord, error) {
	if !opts.Enabled {
		return prompt, baseTokens, nil, nil
	}

	provider := strings.ToLower(strings.TrimSpace(opts.EmbedProvider))
	if provider == "" && cfg != nil && cfg.EmbeddingProvider != "" {
		provider = strings.ToLower(cfg.EmbeddingProvider)
	}
	if provider == "" {
		provider = ai.ProviderOpenRouter
	}

	embedModel := strings.TrimSpace(opts.EmbedModel)
	if embedModel == "" && cfg != nil && cfg.EmbeddingModel != "" {
		embedModel = cfg.EmbeddingModel
	}
	if embedModel == "" {
		if provider == ai.ProviderOllama {
			embedModel = "nomic-embed-text"
		} else {
			embedModel = "openai/text-embedding-3-small"
		}
	}

	if deps.newEmbedder == nil {
		deps.newEmbedder = defaultNewEmbedder
	}
	if deps.buildIndex == nil {
		deps.buildIndex = retrieval.BuildIndex
	}

	emb, err := deps.newEmbedder(ctx, provider, embedModel, cfg, opts)
	if err != nil {
		return "", 0, nil, fmt.Errorf("init embedder: %w", err)
	}

	docs := make(map[string]struct{ Name, Content string }, len(p.Documents))
	for id, d := range p.Documents {
		docs[id] = struct{ Name, Content string }{Name: d.Name, Content: d.Content}
	}

	buildOpts := retrieval.BuildOptions{
		Force:           opts.Reindex,
		EmbedProvider:   provider,
		EmbedModel:      embedModel,
		ChunkMaxTokens:  400,
		ChunkOverlap:    60,
		Include:         nil,
		Exclude:         nil,
		MaxChunksPerDoc: 0,
	}
	if cfg != nil {
		if len(cfg.RetrievalInclude) > 0 {
			buildOpts.Include = cfg.RetrievalInclude
		}
		if len(cfg.RetrievalExclude) > 0 {
			buildOpts.Exclude = cfg.RetrievalExclude
		}
		if cfg.RetrievalMaxChunksPerDoc > 0 {
			buildOpts.MaxChunksPerDoc = cfg.RetrievalMaxChunksPerDoc
		}
	}

	idx, err := deps.buildIndex(ctx, emb, p.RootDir(), docs, buildOpts)
	if err != nil {
		return "", 0, nil, fmt.Errorf("build retrieval index: %w", err)
	}

	vectors, err := emb.Embed(ctx, []string{p.Instructions})
	if err != nil || len(vectors) == 0 {
		return "", 0, nil, fmt.Errorf("embed query: %w", err)
	}

	topK := opts.TopK
	if topK <= 0 && cfg != nil && cfg.RetrievalTopK > 0 {
		topK = cfg.RetrievalTopK
	}
	if topK <= 0 {
		topK = 6
	}

	minScore := opts.MinScore
	if minScore <= 0 && cfg != nil {
		minScore = cfg.RetrievalMinScore
	}
	if minScore < 0 {
		minScore = 0
	}

	records := idx.SearchWithScores(vectors[0], topK, minScore)
	if len(records) == 0 {
		return prompt, baseTokens, nil, nil
	}

	var sb strings.Builder
	sb.WriteString("[RETRIEVED CONTEXT]\n")
	for i, r := range records {
		sb.WriteString(fmt.Sprintf("-- %d) %s (chunk %d) --\n", i+1, r.DocName, r.ChunkID))
		sb.WriteString(r.Text)
		sb.WriteString("\n\n")
	}

	augmented := strings.Replace(prompt, "[REFERENCE DOCUMENTS]", sb.String()+"[REFERENCE DOCUMENTS]", 1)
	return augmented, utils.CountTokens(augmented), records, nil
}

func defaultNewEmbedder(ctx context.Context, provider, model string, cfg *cfgpkg.Global, opts retrievalOptions) (retrieval.Embedder, error) {
	timeout := 60 * time.Second
	if cfg != nil && cfg.HTTPTimeoutSec > 0 {
		timeout = time.Duration(cfg.HTTPTimeoutSec) * time.Second
	}

	switch provider {
	case ai.ProviderOllama:
		host := strings.TrimSpace(opts.OllamaHost)
		if host == "" {
			if v := os.Getenv("SMUSHMUX_OLLAMA_HOST"); v != "" {
				host = v
			}
		}
		if host == "" && cfg != nil && cfg.OllamaHost != "" {
			host = cfg.OllamaHost
		}
		if host == "" {
			host = "http://127.0.0.1:11434"
		}
		if v := os.Getenv("SMUSHMUX_OLLAMA_TIMEOUT_SEC"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				timeout = time.Duration(n) * time.Second
			}
		}
		if cfg != nil && cfg.OllamaTimeoutSec > 0 {
			timeout = time.Duration(cfg.OllamaTimeoutSec) * time.Second
		}
		client := ai.NewOllamaEmbClient(host, timeout)
		return embedFunc(func(ctx context.Context, texts []string) ([][]float32, error) {
			return client.Embed(ctx, model, texts)
		}), nil
	default:
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" && cfg != nil && cfg.APIKey != "" {
			apiKey = cfg.APIKey
		}
		client := ai.NewClient(apiKey, timeout, 3, 500*time.Millisecond, 4*time.Second)
		return embedderAdapter{c: client, model: model}, nil
	}
}

type runtimeOptions struct {
	ProviderFlag string
	OllamaHost   string
}

func buildRuntime(cfg *cfgpkg.Global, opts runtimeOptions) (ai.Runtime, string, error) {
	httpTimeout := 60 * time.Second
	retryMax := 3
	baseDelay := 500 * time.Millisecond
	maxDelay := 4 * time.Second
	if cfg != nil {
		if cfg.HTTPTimeoutSec > 0 {
			httpTimeout = time.Duration(cfg.HTTPTimeoutSec) * time.Second
		}
		if cfg.RetryMaxAttempts > 0 {
			retryMax = cfg.RetryMaxAttempts
		}
		if cfg.RetryBaseDelayMs > 0 {
			baseDelay = time.Duration(cfg.RetryBaseDelayMs) * time.Millisecond
		}
		if cfg.RetryMaxDelayMs > 0 {
			maxDelay = time.Duration(cfg.RetryMaxDelayMs) * time.Millisecond
		}
	}

	providerName := strings.ToLower(strings.TrimSpace(opts.ProviderFlag))
	if providerName == "" && cfg != nil && cfg.DefaultProvider != "" {
		providerName = strings.ToLower(cfg.DefaultProvider)
	}
	if providerName == "" {
		providerName = ai.ProviderOpenRouter
	}

	switch providerName {
	case "local":
		providerName = ai.ProviderOllama
	case "openai", "anthropic", "google", "gemini", "meta", "llama":
		providerName = ai.ProviderOpenRouter
	case "ollama":
		providerName = ai.ProviderOllama
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" && cfg != nil && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}

	rc := ai.RuntimeConfig{
		HTTPTimeout: httpTimeout,
		RetryMax:    retryMax,
		BaseDelay:   baseDelay,
		MaxDelay:    maxDelay,
		APIKey:      apiKey,
	}

	if providerName == ai.ProviderOllama {
		host := strings.TrimSpace(opts.OllamaHost)
		if host == "" {
			if v := os.Getenv("SMUSHMUX_OLLAMA_HOST"); v != "" {
				host = v
			}
		}
		if host == "" && cfg != nil && cfg.OllamaHost != "" {
			host = cfg.OllamaHost
		}
		if host == "" {
			host = "http://127.0.0.1:11434"
		}
		rc.Host = host
		if v := os.Getenv("SMUSHMUX_OLLAMA_TIMEOUT_SEC"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				rc.HTTPTimeout = time.Duration(n) * time.Second
			}
		}
		if cfg != nil && cfg.OllamaTimeoutSec > 0 {
			rc.HTTPTimeout = time.Duration(cfg.OllamaTimeoutSec) * time.Second
		}
	}

	client, ok := ai.GetRuntime(providerName, rc)
	if !ok {
		return nil, providerName, fmt.Errorf("provider not supported: %s", providerName)
	}
	return client, providerName, nil
}

func selectModel(p *project.Project, cfg *cfgpkg.Global, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if p != nil && p.Config != nil && p.Config.Model != "" {
		return p.Config.Model
	}
	if cfg != nil && cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return "openai/gpt-4o-mini"
}

func enforceBudget(estCost, limit float64) error {
	if limit > 0 && estCost > 0 && estCost > limit {
		return fmt.Errorf("✗ Estimated cost ~$%.4f exceeds budget limit ~$%.4f", estCost, limit)
	}
	return nil
}

type streamingOptions struct {
	Enabled     bool
	Quiet       bool
	PrintPrompt bool
	Prompt      string
	Writer      io.Writer
	DeltaWriter io.Writer
}

func handleStreaming(ctx context.Context, runtime ai.Runtime, req ai.GenerateRequest, opts streamingOptions) (bool, error) {
	if !opts.Enabled {
		return false, nil
	}

	logWriter := opts.Writer
	if logWriter == nil {
		logWriter = os.Stdout
	}
	deltaWriter := opts.DeltaWriter
	if deltaWriter == nil {
		deltaWriter = os.Stdout
	}

	sr, ok := runtime.(ai.StreamRuntime)
	if !ok {
		if !opts.Quiet {
			fmt.Fprintln(logWriter, "⚠ Streaming not supported for this provider; falling back to non-streaming.")
		}
		return false, nil
	}

	if opts.PrintPrompt && !opts.Quiet {
		fmt.Fprintln(logWriter, "\n--print-prompt: sending the following prompt --")
		fmt.Fprintln(logWriter, opts.Prompt)
	}
	if !opts.Quiet {
		fmt.Fprintln(logWriter, "(streaming)")
	}

	if err := sr.GenerateStream(ctx, req, func(delta string) {
		fmt.Fprint(deltaWriter, delta)
	}); err != nil {
		return true, fmt.Errorf("streaming generation failed: %w", err)
	}

	if !opts.Quiet {
		fmt.Fprintln(logWriter)
	}
	return true, nil
}

type outputOptions struct {
	JSON         bool
	Quiet        bool
	Project      string
	Model        string
	MaxTokens    int
	Temperature  float64
	PromptTokens int
	OutputPath   string
	OutputFormat string
	Writer       io.Writer
}

func formatAndWriteOutput(content string, opts outputOptions) error {
	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}

	if opts.JSON {
		out := map[string]any{
			"project":       opts.Project,
			"model":         opts.Model,
			"max_tokens":    opts.MaxTokens,
			"temperature":   opts.Temperature,
			"prompt_tokens": opts.PromptTokens,
			"content":       content,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal output: %w", err)
		}
		fmt.Fprintln(w, string(b))
	} else {
		if opts.Quiet {
			fmt.Fprintln(w, content)
		} else {
			fmt.Fprintln(w, "\n=== AI Response ===")
			fmt.Fprintln(w, content)
		}
	}

	if opts.OutputPath == "" {
		return nil
	}

	switch opts.OutputFormat {
	case "", "text", "markdown", "md":
		if err := os.WriteFile(opts.OutputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	case "json":
		out := map[string]any{
			"project":       opts.Project,
			"model":         opts.Model,
			"max_tokens":    opts.MaxTokens,
			"temperature":   opts.Temperature,
			"prompt_tokens": opts.PromptTokens,
			"content":       content,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal output: %w", err)
		}
		if err := os.WriteFile(opts.OutputPath, b, 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	default:
		return fmt.Errorf("unsupported --format: %s (use text|markdown|json)", opts.OutputFormat)
	}

	if !opts.Quiet {
		fmt.Fprintf(w, "\n💾 Saved output to %s\n", opts.OutputPath)
	}
	return nil
}
