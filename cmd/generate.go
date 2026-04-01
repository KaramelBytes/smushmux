package cmd

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/KaramelBytes/smushmux/internal/ai"
	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
	"github.com/KaramelBytes/smushmux/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// embedderAdapter adapts ai.Client to retrieval.Embedder with a fixed model name.
type embedderAdapter struct {
	c     *ai.Client
	model string
}

func (e embedderAdapter) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return e.c.Embed(ctx, e.model, texts)
}

// embedFunc adapts a function to retrieval.Embedder.
type embedFunc func(context.Context, []string) ([][]float32, error)

func (f embedFunc) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return f(ctx, texts)
}

var (
	genProjectName string
	genModel       string
	genModelPreset string
	genProvider    string
	genMaxTokens   int
	genTemp        float64
	genDryRun      bool
	genQuiet       bool
	genJSON        bool
	genPrintPrompt bool
	genPromptLimit int
	genBudgetLimit float64
	genOutputPath  string
	genOutputFmt   string
	genStream      bool
	genOllamaHost  string
	genTimeoutSec  int
	genExplain     bool
	// Retrieval flags
	genRetrieval       bool
	genReindex         bool
	genEmbedModel      string
	genEmbedProvider   string
	genRetrievalTopK   int
	genRetrievalMinSim float64
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an AI response from the project's documents and instructions",
	Example: `  smushmux generate -p myproj --dry-run
	smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 512
	smushmux generate -p myproj --budget-limit 0.05 --prompt-limit 60000
	smushmux generate -p myproj --output out.md --format markdown`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if genProjectName == "" {
			return fmt.Errorf("--project is required")
		}

		if genJSON {
			genQuiet = true
		}

		// Ensure flags that can carry over between invocations are reset to defaults
		// unless explicitly provided in THIS run. Use Visit to detect set flags in this parse.
		if f := cmd.Flags(); f != nil {
			provided := map[string]bool{}
			f.Visit(func(fl *pflag.Flag) {
				provided[fl.Name] = true
			})
			if !provided["budget-limit"] {
				genBudgetLimit = 0
			}
			if !provided["prompt-limit"] {
				genPromptLimit = 0
			}
			if !provided["print-prompt"] {
				genPrintPrompt = false
			}
			if !provided["provider"] {
				genProvider = ""
			}
			if !provided["model"] {
				genModel = ""
			}
			if !provided["max-tokens"] {
				genMaxTokens = 0
			}
			if !provided["timeout-sec"] {
				genTimeoutSec = 180
			}
			if !provided["dry-run"] {
				genDryRun = false
			}
			if !provided["explain"] {
				genExplain = false
			}
		}

		projDir, err := resolveProjectDirByName(genProjectName)
		if err != nil {
			return err
		}
		p, err := project.LoadProject(projDir)
		if err != nil {
			return err
		}
		// Apply provider-preset via explicit --provider (offline, no network)
		providerUsed := ""
		if genProvider != "" {
			if preset, ok := ai.PresetCatalog(genProvider); ok {
				ai.MergeCatalog(preset)
				providerUsed = genProvider
				fmt.Printf("Using built-in provider preset: %s (merged into catalog)\n", genProvider)
			} else {
				return fmt.Errorf("unknown --provider: %s (try openrouter|openai|anthropic|google|gemini|meta|llama)", genProvider)
			}
		}

		// Apply provider and/or tier presets via --model-preset if requested (offline, no network)
		if genModelPreset != "" {
			provider := genModelPreset
			tier := ""
			if strings.Contains(genModelPreset, ":") {
				parts := strings.SplitN(genModelPreset, ":", 2)
				provider, tier = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			} else {
				// If value matches a known tier, keep provider empty to use defaults
				switch genModelPreset {
				case "cheap", "balanced", "high-context":
					provider = ""
					tier = genModelPreset
				}
			}
			if provider != "" {
				if preset, ok := ai.PresetCatalog(provider); ok {
					ai.MergeCatalog(preset)
					providerUsed = provider
					fmt.Printf("Using built-in provider preset: %s (merged into catalog)\n", provider)
				} else if tier == "" { // only error if neither provider nor tier recognized
					return fmt.Errorf("unknown --model-preset: %s (try openrouter|openai|anthropic or :cheap|:balanced|:high-context)", genModelPreset)
				}
			}
			if tier != "" && genModel == "" {
				// Prefer explicitly set --provider if any
				prov := providerUsed
				if prov == "" && genProvider != "" {
					prov = genProvider
				}
				if name, ok := ai.RecommendModel(prov, tier); ok {
					genModel = name
					if prov == "" {
						prov = "default"
					}
					fmt.Printf("Selected model by tier preset (%s:%s): %s\n", prov, tier, name)
				} else {
					return fmt.Errorf("unknown tier in --model-preset: %s (use cheap|balanced|high-context)", tier)
				}
			}
		}
		prompt, tokens, err := p.BuildPrompt()
		if err != nil {
			return err
		}

		var retrievalRecords []retrieval.ScoredRecord
		prompt, tokens, retrievalRecords, err = prepareRetrievedContext(
			cmd.Context(),
			p,
			prompt,
			tokens,
			cfg,
			retrievalOptions{
				Enabled:       genRetrieval,
				Reindex:       genReindex,
				EmbedModel:    genEmbedModel,
				EmbedProvider: genEmbedProvider,
				TopK:          genRetrievalTopK,
				MinScore:      genRetrievalMinSim,
				OllamaHost:    genOllamaHost,
			},
			defaultRetrievalDeps,
		)
		if err != nil {
			return err
		}

		// Optional prompt cap/truncation before proceeding
		if genPromptLimit > 0 && tokens > genPromptLimit {
			if !genQuiet {
				fmt.Printf("⚠ Prompt exceeds limit (%d > %d). Truncating before send...\n", tokens, genPromptLimit)
			}
			prompt = utils.TruncateToTokenLimit(prompt, genPromptLimit)
			tokens = utils.CountTokens(prompt)
		}

		model := selectModel(p, cfg, genModel)

		maxTokens := genMaxTokens
		if maxTokens == 0 && p.Config != nil && p.Config.MaxTokens > 0 {
			maxTokens = p.Config.MaxTokens
		}
		if maxTokens == 0 {
			maxTokens = 1024
		}

		temp := genTemp
		if temp == 0 && p.Config != nil && p.Config.Temperature > 0 {
			temp = p.Config.Temperature
		}
		if temp == 0 {
			temp = 0.7
		}

		// Token breakdown
		docsTokens := 0
		for _, d := range p.Documents {
			docsTokens += d.Tokens
		}
		instrTokens := utils.CountTokens(p.Instructions)
		overhead := tokens - (docsTokens + instrTokens)
		if overhead < 0 {
			overhead = 0
		}

		if !genQuiet {
			fmt.Printf("Tokens: total≈%d (instructions≈%d, docs≈%d, overhead≈%d)\n", tokens, instrTokens, docsTokens, overhead)
		}

		// Model metadata and pricing warnings
		var estCost float64
		if mi, ok := ai.LookupModel(model); ok {
			
			// Override ContextTokens if MaxContextCap is configured
			contextLimit := mi.ContextTokens
			if cfg.MaxContextCap > 0 && cfg.MaxContextCap < mi.ContextTokens {
				contextLimit = cfg.MaxContextCap
			}
			
			if !genQuiet {
				fmt.Printf("DEBUG: Model: %s, ContextTokens: %d, tokens: %d, maxTokens: %d\n", mi.Name, contextLimit, tokens, maxTokens)
			}
			if !genDryRun && (tokens+maxTokens > contextLimit) {
				msg := fmt.Sprintf("⚠ Prompt (%d tokens) + max-tokens (%d) exceeds %s context window (~%d tokens).\n",
					tokens, maxTokens, mi.Name, contextLimit)

				if !genQuiet {
					fmt.Print(msg)
				}

				{
					_, providerName, err := buildRuntime(cfg, runtimeOptions{
						ProviderFlag: genProvider,
						OllamaHost:   genOllamaHost,
					})
					if err != nil {
						return err
					}
					if providerName == ai.ProviderOllama || providerName == "local" {
						availableForPrompt := contextLimit - maxTokens
						if availableForPrompt < 0 {
							availableForPrompt = contextLimit / 2 // Conservative
						}

						return fmt.Errorf("context window exceeded for local model '%s'.\n"+
							"  Required: %d tokens (prompt) + %d (max-tokens) = %d total\n"+
							"  Available: %d tokens%s\n\n"+
							"Solutions:\n"+
							"  1. Use --prompt-limit %d to truncate the prompt\n"+
							"  2. Enable retrieval mode with --retrieval to use only relevant chunks\n"+
							"  3. Remove documents from project or reduce --max-rows for XLSX files\n"+
							"  4. Increase max_context_cap in config if your system has enough RAM",
							model, tokens, maxTokens, tokens+maxTokens, availableForPrompt,
							func() string { if cfg.MaxContextCap > 0 { return " (limited by max_context_cap)" } else { return "" } }(),
							availableForPrompt)
					}
				}
			}
			if cost, ok := ai.EstimateCostUSD(model, tokens, maxTokens); ok {
				estCost = cost
				if !genQuiet {
					fmt.Printf("Estimated max cost: ~$%.4f (in %.4f/out %.4f per 1K tokens)\n", cost, mi.InputPerK, mi.OutputPerK)
				}
			}
		}

		if err := enforceBudget(estCost, genBudgetLimit); err != nil {
			return err
		}

		if genExplain {
			explainWriter := os.Stdout
			if genJSON {
				explainWriter = os.Stderr
			}
			// Resolve display values for the report.
			displayProvider := genProvider
			if displayProvider == "" && cfg != nil && cfg.DefaultProvider != "" {
				displayProvider = cfg.DefaultProvider
			}
			resolvedEmbedModel := genEmbedModel
			if resolvedEmbedModel == "" && cfg != nil && cfg.EmbeddingModel != "" {
				resolvedEmbedModel = cfg.EmbeddingModel
			}
			resolvedEmbedProvider := genEmbedProvider
			if resolvedEmbedProvider == "" && cfg != nil && cfg.EmbeddingProvider != "" {
				resolvedEmbedProvider = cfg.EmbeddingProvider
			}
			maxChunksPerDoc := 0
			if cfg != nil && cfg.RetrievalMaxChunksPerDoc > 0 {
				maxChunksPerDoc = cfg.RetrievalMaxChunksPerDoc
			}
			report := buildEvidenceReport(
				p,
				projDir,
				displayProvider,
				model,
				genModelPreset,
				maxTokens,
				tokens,
				genPromptLimit,
				genBudgetLimit,
				estCost,
				retrievalOptions{
					Enabled:         genRetrieval,
					TopK:            genRetrievalTopK,
					MinScore:        genRetrievalMinSim,
					EmbedModel:      resolvedEmbedModel,
					EmbedProvider:   resolvedEmbedProvider,
					MaxChunksPerDoc: maxChunksPerDoc,
				},
				retrievalRecords,
			)
			renderEvidenceReport(report, explainWriter)
		}

		if genDryRun {
			if !genQuiet {
				// Deterministic dry-run request id for observability
				sum := sha1.Sum([]byte(prompt))
				rid := fmt.Sprintf("sim_%x", sum[:6])
				fmt.Println("\n--dry-run: no API call will be made. Prompt preview below --")
				fmt.Printf("Request ID (dry-run): %s\n", rid)
			}
			fmt.Println(prompt)
			return nil
		}

		if genPrintPrompt && !genQuiet {
			fmt.Println("\n--print-prompt: sending the following prompt --")
			fmt.Println(prompt)
		}

		client, providerName, err := buildRuntime(cfg, runtimeOptions{
			ProviderFlag: genProvider,
			OllamaHost:   genOllamaHost,
		})
		if err != nil {
			return err
		}

		// Request timeout
		timeoutSec := genTimeoutSec
		if timeoutSec <= 0 {
			timeoutSec = 180
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()

		req := ai.GenerateRequest{
			Model: model,
			Messages: []ai.Message{
				{Role: "user", Content: prompt},
			},
			MaxTokens:   maxTokens,
			Temperature: temp,
		}

		// Basic warning if prompt is very large relative to typical limits
		if tokens > 100000 {
			if !genQuiet {
				fmt.Printf("⚠ Warning: very large prompt (≈%d tokens). Consider removing or truncating documents.\n", tokens)
			}
		}
		if genMaxTokens > 0 && (tokens+genMaxTokens) > 120000 {
			if !genQuiet {
				fmt.Printf("⚠ Warning: prompt + max-tokens (≈%d) may exceed common model context windows.\n", tokens+genMaxTokens)
			}
		}

		if !genQuiet {
			fmt.Printf("⚙ Generating with model=%s (prompt tokens≈%d) ...\n", model, tokens)
		}
		handled, err := handleStreaming(ctx, client, req, streamingOptions{
			Enabled:     genStream,
			Quiet:       genQuiet,
			PrintPrompt: genPrintPrompt,
			Prompt:      prompt,
			Writer:      os.Stdout,
			DeltaWriter: os.Stdout,
		})
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		resp, err := client.Generate(ctx, req)
		if err != nil {
			// Provide user-friendly hints for common error classes
			var (
				authErr *ai.AuthError
				rlErr   *ai.RateLimitError
				nfErr   *ai.ModelNotFoundError
				brErr   *ai.BadRequestError
				qErr    *ai.QuotaExceededError
				sErr    *ai.ServerError
				unreach *ai.UnreachableError
			)
			switch {
			case errors.As(err, &unreach):
				if providerName == ai.ProviderOllama {
					return fmt.Errorf("Ollama not reachable at %s. Ensure Ollama is running (see https://ollama.com) and host is correct. You can set SMUSHMUX_OLLAMA_HOST or config 'ollama_host'. Detail: %w", unreach.Host, err)
				}
				return fmt.Errorf("endpoint unreachable. Check your network and provider settings: %w", err)
			case errors.As(err, &authErr):
				return fmt.Errorf("authentication failed: set OPENROUTER_API_KEY or add api_key in config (~/.smushmux/config.yaml): %w", err)
			case errors.As(err, &rlErr):
				if rlErr.RetryAfter > 0 {
					return fmt.Errorf("rate limited, try again in ~%ds: %w", int(rlErr.RetryAfter.Seconds()), err)
				}
				return fmt.Errorf("rate limited by provider, please retry: %w", err)
			case errors.As(err, &nfErr):
				if providerName == ai.ProviderOllama {
					return fmt.Errorf("local model not available (%s). Install it with 'ollama pull %s' or choose another model. %w", model, model, err)
				}
				return fmt.Errorf("model not found (%s). Verify the model name or sync catalog via 'smushmux models fetch' or 'smushmux models show': %w", model, err)
			case errors.As(err, &brErr):
				// Check if prompt was very large
				if tokens > 50000 {
					return fmt.Errorf("request invalid: prompt is very large (%d tokens).\n"+
						"  This often happens with multiple XLSX files in a project.\n"+
						"  Try: --retrieval mode (processes only relevant chunks), or reduce documents",
						tokens)
				}
				return fmt.Errorf("request invalid. Try reducing prompt size or max-tokens: %w", err)
			case errors.As(err, &qErr):
				return fmt.Errorf("quota/billing issue. Check your provider account: %w", err)
			case errors.As(err, &sErr):
				return fmt.Errorf("provider appears unavailable (server error). Please retry later: %w", err)
			default:
				return fmt.Errorf("generation failed: %w", err)
			}
		}
		if len(resp.Choices) == 0 {
			return fmt.Errorf("no content returned from model")
		}
		if resp.RequestID != "" {
			fmt.Printf("Request ID: %s\n", resp.RequestID)
		}
		content := resp.Choices[0].Message.Content
		if err := formatAndWriteOutput(content, outputOptions{
			JSON:         genJSON,
			Quiet:        genQuiet,
			Project:      genProjectName,
			Model:        model,
			MaxTokens:    maxTokens,
			Temperature:  temp,
			PromptTokens: tokens,
			OutputPath:   genOutputPath,
			OutputFormat: genOutputFmt,
			Writer:       os.Stdout,
		}); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&genProjectName, "project", "p", "", "project name")
	generateCmd.Flags().StringVar(&genModel, "model", "", "override model (default from project config)")
	generateCmd.Flags().StringVar(&genModelPreset, "model-preset", "", "apply preset: provider catalog (openrouter|openai|anthropic|google|gemini|meta|llama) or tier (cheap|balanced|high-context) or <provider>:<tier>")
	generateCmd.Flags().StringVar(&genProvider, "provider", "", "explicit provider to merge catalog and guide tier selection (openrouter|openai|anthropic|google|gemini|meta|llama)")
	generateCmd.Flags().IntVar(&genMaxTokens, "max-tokens", 0, "max tokens for response")
	generateCmd.Flags().Float64Var(&genTemp, "temp", 0, "sampling temperature")
	generateCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "build prompt and print token breakdown without calling the API")
	generateCmd.Flags().BoolVar(&genPrintPrompt, "print-prompt", false, "print the prompt being sent to the API")
	generateCmd.Flags().IntVar(&genPromptLimit, "prompt-limit", 0, "truncate built prompt to this many tokens before sending")
	generateCmd.Flags().Float64Var(&genBudgetLimit, "budget-limit", 0, "fail if estimated max cost (USD) exceeds this budget")
	generateCmd.Flags().StringVar(&genOutputPath, "output", "", "optional path to write the response (skips in --dry-run)")
	generateCmd.Flags().StringVar(&genOutputFmt, "format", "text", "output format: text|markdown|json")
	generateCmd.Flags().BoolVar(&genQuiet, "quiet", false, "suppress non-essential output")
	generateCmd.Flags().BoolVar(&genJSON, "json", false, "emit response as JSON to stdout")
	generateCmd.Flags().BoolVar(&genStream, "stream", false, "stream responses if supported by the provider")
	generateCmd.Flags().StringVar(&genOllamaHost, "ollama-host", "", "override Ollama host (e.g., http://127.0.0.1:11434)")
	generateCmd.Flags().IntVar(&genTimeoutSec, "timeout-sec", 180, "request timeout in seconds (default 180)")
	generateCmd.Flags().BoolVar(&genExplain, "explain", false, "print a human-readable evidence report summarizing the run inputs")
	// Retrieval flags
	generateCmd.Flags().BoolVar(&genRetrieval, "retrieval", false, "enable retrieval-augmented generation (RAG)")
	generateCmd.Flags().BoolVar(&genReindex, "reindex", false, "rebuild the retrieval index before generation")
	generateCmd.Flags().StringVar(&genEmbedModel, "embed-model", "", "embedding model for retrieval (e.g., openai/text-embedding-3-small or nomic-embed-text)")
	generateCmd.Flags().StringVar(&genEmbedProvider, "embed-provider", "", "embedding provider: openrouter|ollama")
	generateCmd.Flags().IntVar(&genRetrievalTopK, "top-k", 6, "number of chunks to retrieve for context")
	generateCmd.Flags().Float64Var(&genRetrievalMinSim, "min-score", 0.0, "minimum cosine similarity threshold for retrieved chunks")
}
