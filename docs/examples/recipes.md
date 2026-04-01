# Recipes: Common Flows and Tips

Practical command combinations for SmushMux. Replace `myproj` with your project name. Many flows work offline with `--dry-run`.

## 1) Quick summary from mixed docs
```bash
smushmux init myproj -d "Quick summary"
smushmux add -p myproj ./CHANGELOG.md --desc "Changelog"
smushmux add -p myproj ./README.md --desc "Overview"
smushmux instruct -p myproj "Summarize key changes and provide an overview paragraph."
smushmux generate -p myproj --dry-run --print-prompt
# Real run
export OPENROUTER_API_KEY=your_key
smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 400
```

## 2) Budget-guarded run with prompt cap
```bash
smushmux generate -p myproj \
  --prompt-limit 60000 \
  --budget-limit 0.03 \
  --model openai/gpt-4o-mini \
  --max-tokens 512
```

## 3) Pick a model by tier preset
```bash
# Choose a low-cost default
smushmux generate -p myproj --model-preset cheap

# Provider + tier
smushmux generate -p myproj --provider anthropic --model-preset balanced
```

## 4) Use Ollama locally (no network)
```bash
# Ensure Ollama is running and a chat-capable model is installed (e.g., llama3)
smushmux generate -p myproj --provider ollama --model llama3:latest --dry-run

smushmux generate -p myproj --provider ollama --model llama3:latest --stream --max-tokens 256
```

## 5) Retrieval (OpenRouter embeddings)
```bash
# Default embedding provider is openrouter; this builds/refreshes index.json
smushmux generate -p myproj --retrieval --embed-model openai/text-embedding-3-small --top-k 8 --min-score 0.2 --dry-run
```

## 6) Retrieval (Ollama embeddings)
```bash
smushmux generate -p myproj --retrieval --embed-provider ollama --embed-model nomic-embed-text \
  --model llama3:latest --top-k 6 --min-score 0.25 --dry-run
```

## 7) Output control
```bash
# Save markdown output
smushmux generate -p myproj --output out.md --format markdown

# Emit JSON to stdout (for scripts)
smushmux generate -p myproj --json --quiet | jq .
```

## 8) Instruction templates
```bash
# Example: concise-summary template
smushmux instruct -p myproj "$(cat docs/templates/concise-summary.md)"
smushmux generate -p myproj --dry-run
```

## 9) Catalog hygiene
```bash
# Inspect the in-memory catalog
smushmux models show | jq .

# Merge a provider preset (offline)
smushmux models fetch --provider openrouter --merge --output models-openrouter.json

# Replace from a local file
smushmux models sync --file models-openrouter.json
```
