# Quickstart: Create, Add, Instruct, Generate

This walkthrough shows a minimal end‑to‑end flow using SmushMux.

1) Initialize a project

```bash
smushmux init myproj -d "Docs to merge"
```

2) Add documents

```bash
smushmux add -p myproj ./README.md --desc "Main readme"
# add more docs as needed; supports .txt, .md, .docx, .csv, .tsv, .xlsx

# Tip: CSV/TSV/XLSX are summarized instead of printed raw.
# You can pre-check or export a summary with:
#   smushmux analyze ./data/metrics.csv --output metrics_summary.md
# For many files at once (with progress):
#   smushmux analyze-batch "data/*.csv"
# When attaching (-p), control samples across all outputs:
#   --sample-rows-project 0   # disable sample tables

## Optional: Use analysis instructions

# Include an instruction template to guide the model’s interpretation of the dataset summary:
smushmux add -p myproj docs/templates/dataset-analysis.md --desc "Analysis Instructions"

# Or set the project instructions from the file content:
# smushmux instruct -p myproj "$(cat docs/templates/dataset-analysis.md)"
```

3) Set instructions

```bash
smushmux instruct -p myproj "Summarize key features and provide a short overview."
```

4) (Optional) Enable retrieval for context-aware prompts

```bash
# OpenRouter embeddings (defaults to instructions + top 6 chunks)
smushmux generate -p myproj --retrieval --embed-model openai/text-embedding-3-small --dry-run

# Local embeddings via Ollama (requires an embedding model such as nomic-embed-text)
smushmux generate -p myproj --retrieval --embed-provider ollama --embed-model nomic-embed-text --dry-run
```

5) Dry run to inspect prompt and token breakdown (no API call)

```bash
smushmux generate -p myproj --dry-run --print-prompt
```

6) Preview Evidence Mode output (offline, no API call)

```bash
smushmux generate -p myproj --dry-run --explain
```

7) Real run (requires API key)

```bash
export OPENROUTER_API_KEY=your_key
smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 512
```
