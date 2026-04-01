# SmushMux CLI

> 🎩 **Legacy Notice**: This project represents "SmushMux v1", a single-shot, statistical-summarizing document merger initially built in 2025 for a plant geneticist analyzing dense CSV/XLSX data. A final  polish pass was completed and development locked in early 2026. To support operation on memory-constrained local machines, we explicitly set parser column/row limits and optionally cap global token contexts to avoid swapping. We know enough to do it better now—future tracking and improvements will occur in the agentic SmushMux vNext.
> 🔒 **Archive Status**: This repository is read-only and unmaintained.

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

SmushMux is a Go CLI that merges multiple documents into a unified, AI-ready context and sends it to models via OpenRouter or Ollama for analysis, synthesis, and content generation.

- MVP focus: stateless, single-shot generation
- Formats: .txt, .md, .docx, .csv, .tsv, .xlsx (tabular files are summarized automatically)
- Retrieval: optional embedding index per project with OpenRouter or Ollama embeddings
- Cross-platform builds: Linux, macOS, Windows
- Local-friendly: first-class Ollama runtime support, streaming, and model presets

## Install / Build

Prerequisites: Go 1.22+

```bash
git clone https://github.com/KaramelBytes/smushmux
cd smushmux

# Build a local binary in the current directory
go build -o smushmux .

# Or run directly during development
go run . --help
```

### Direct Downloads
- Linux (amd64): https://github.com/KaramelBytes/smushmux/releases/latest/download/smushmux-linux-amd64
- macOS (Intel): https://github.com/KaramelBytes/smushmux/releases/latest/download/smushmux-darwin-amd64
- macOS (Apple Silicon): https://github.com/KaramelBytes/smushmux/releases/latest/download/smushmux-darwin-arm64
- Windows (amd64, exe): https://github.com/KaramelBytes/smushmux/releases/latest/download/smushmux-windows-amd64.exe
- Checksums: https://github.com/KaramelBytes/smushmux/releases/latest/download/checksums.txt

### Install (alternative)
- Linux/macOS script (downloads latest release):
  ```bash
  # Install to a user-local bin dir (recommended)
  curl -fsSL https://raw.githubusercontent.com/KaramelBytes/smushmux/main/scripts/install.sh | BIN_DIR="$HOME/.local/bin" bash

  # Or specify a version explicitly
  VERSION=v0.1.0 BIN_DIR="$HOME/.local/bin" bash <(curl -fsSL https://raw.githubusercontent.com/KaramelBytes/smushmux/main/scripts/install.sh)
  ```
  Notes:
  - If installing to `/usr/local/bin`, you may need `sudo`.
  - Ensure your chosen `BIN_DIR` is on `PATH`.
  - After install, run `smushmux --help`.

#### Config-driven auto-sync

You can optionally auto-sync the catalog on startup via `~/.smushmux/config.yaml`:

```yaml
models_auto_sync: true            # default false
models_merge: true                # default true; if false, replace
models_catalog_url: "https://raw.githubusercontent.com/KaramelBytes/smushmux/main/docs/openrouter-models.json" # optional direct URL
models_provider: "openrouter"     # optional provider preset if URL not set
max_context_cap: 4096             # strictly enforce max context tokens across local models to avoid swapping
```

If both `models_catalog_url` and `models_provider` are set, the URL takes precedence.

Or build cross-platform (host-default; set TARGETS to override):

```bash
# Build only for the host (default)
./scripts/build.sh

# Build multiple targets (pure-Go, CGO disabled by default)
TARGETS="linux/amd64 darwin/arm64 windows/amd64" ./scripts/build.sh

# Artifacts are written under ./dist
```

## Quick Start

Note: The commands below assume the `smushmux` binary is installed. If you are running from source without installing, replace `smushmux` with `go run .` in the commands.

1) Initialize a project

```bash
smushmux init myproj -d "Docs to merge"
```

2) Add documents

```bash
smushmux add -p myproj ./docs/example.md --desc "Example"
```

3) Set instructions

```bash
smushmux instruct -p myproj "Summarize the key points"
```

4) Generate (requires API key)

```bash
export OPENROUTER_API_KEY=your_key_here
smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 512
```

5) Dry run and token breakdown (no API call)

```bash
smushmux generate -p myproj --dry-run
```

- **Smoke Test**
  - Run: `bash scripts/smoke_test.sh`
  - What it does: uses a temporary `HOME`, runs `init` → `add` → `instruct`, then a dry-run `generate` (offline) — no provider calls.
- Optional:
  - If `OPENROUTER_API_KEY` is set, set `SMOKE_TRY_OPENROUTER_RUN=1` to perform a short real generate.
  - Optional OpenRouter tuning:
    - `SMOKE_OPENROUTER_MODEL=<name>` to force a specific model
    - `SMOKE_OPENROUTER_TIER=cheap|balanced|high-context` (default: cheap) to pick by tier preset
    - `SMOKE_OPENROUTER_PROVIDER=openrouter|openai|anthropic|google|gemini|meta|llama` to guide tier selection
    - `SMOKE_OPENROUTER_BUDGET=0.02` to cap the test’s estimated max cost
  - If Ollama is reachable (validated via `/api/tags` JSON), performs a local dry‑run only by default; set `SMOKE_TRY_OLLAMA_RUN=1` to try a short real run.
  - Override the Ollama model via `SMOKE_OLLAMA_MODEL=<name>`; otherwise the script selects a reasonable installed model (prefers `*instruct` variants such as `mistral:7b-instruct`, `llama3:*instruct`, or falls back to `phi3`, `tinyllama`, `gemma2`).
- Does not modify your real config or projects; cleans up after itself.

## Configuration

SmushMux reads configuration from (in order): CLI flags, environment, config file, defaults.

- Environment variables: `SMUSHMUX_*` and `OPENROUTER_API_KEY`
- Config file: `~/.smushmux/config.yaml`

Example `~/.smushmux/config.yaml`:

```yaml
api_key: ""
default_model: "openai/gpt-4o-mini"
default_provider: "openrouter"   # or "ollama" to default to local runtime
max_tokens: 4096
temperature: 0.7
projects_dir: "~/.smushmux/projects"
# HTTP/Retry tuning (optional)
http_timeout_sec: 60            # HTTP client timeout
retry_max_attempts: 3           # API call retries on 429/5xx
retry_base_delay_ms: 500        # initial backoff in ms
retry_max_delay_ms: 4000        # max backoff cap in ms
```

## CLI Overview

```bash
smushmux init <project-name>
  # Creates a new project under ~/.smushmux/projects/<name>

smushmux add -p <project-name> <file> [--desc "..."]
  # Adds a document

smushmux instruct -p <project-name> "..."
  # Sets instructions

smushmux analyze <file> [-p <project-name>] [--output <file>] [--delimiter ','|'tab'|';'] [--decimal '.'|'comma'] [--thousands ','|'.'|'space'] [--sample-rows N] [--max-rows N]
  # Analyzes CSV/TSV/XLSX and produces a compact Markdown summary; can attach to a project
  # Extras: --group-by <col1,col2> --correlations --corr-per-group --outliers --outlier-threshold 3.5 --sheet-name <name> --sheet-index N

smushmux analyze-batch <files...> [-p <project-name>] [--delimiter ...] [--decimal ...] [--thousands ...] [--sample-rows N] [--max-rows N] [--quiet]
  # Analyze multiple CSV/TSV/XLSX files with progress [N/Total]. Supports globs. Mirrors flags from 'analyze'.
  # When attaching (-p), you can override sample rows for all summaries using --sample-rows-project (0 disables samples).

smushmux list --projects | --docs -p <project-name>
  # Lists projects or documents

smushmux generate -p <project-name> [--model ...] [--provider openrouter|openai|anthropic|google|gemini|meta|llama|ollama|local] [--model-preset openrouter|openai|anthropic|google|gemini|meta|llama|cheap|balanced|high-context|<provider>:<tier>] [--max-tokens N] [--temp F] [--dry-run] [--quiet] [--json] [--explain] [--print-prompt] [--prompt-limit N] [--budget-limit USD] [--output <file>] [--format text|markdown|json] [--stream]
  # Builds prompt and sends to OpenRouter (unless --dry-run)

smushmux models show
  # Prints the current in-memory model catalog and pricing as JSON

smushmux models sync --file ./models.json [--merge]
  # Loads a JSON catalog and replaces (default) or merges (with --merge) into the catalog

smushmux models fetch --url https://raw.githubusercontent.com/KaramelBytes/smushmux/main/docs/openrouter-models.json [--merge] [--output models.json]
  # Fetches a remote JSON catalog, optionally saves to a file, and merges/replaces the in-memory catalog

smushmux models fetch --provider openrouter [--merge] [--output models.json]
  # Uses a provider preset; built-in presets can be applied without network
```

## Data Analysis (CSV/TSV/XLSX)

- Purpose: Quickly summarize tabular data into a compact Markdown report with schema inference, basic stats, optional grouping, correlations, and outliers.
- File types: `.csv`, `.tsv`, `.xlsx` (select sheet via `--sheet-name` or `--sheet-index`).
- Delimiters: auto-detects comma, semicolon, tab, and pipe (override via `--delimiter`).
- Behavior in projects: When you `add` CSV/TSV/XLSX to a project, the parser stores a summary (not the raw table) to keep prompts concise and token‑efficient.
- Standalone analysis: Use `smushmux analyze <file>` to generate a report and optionally save it to a file or attach it to a project with `-p`.

Batch analysis with progress

- Use `smushmux analyze-batch "data/*.csv"` (supports globs) to process multiple files with `[N/Total]` progress.
- Supports mixed inputs: `.csv`, `.tsv`, `.xlsx` are analyzed; other formats (`.yaml`, `.md`, `.txt`, `.docx`) are added as regular documents when `-p` is provided.
- When attaching (`-p`), you can override sample rows for all summaries using `--sample-rows-project`. Set it to `0` to disable sample tables in reports.
- When writing summaries into a project (`dataset_summaries/`), filenames are disambiguated:
  - If `--sheet-name` is used, the sheet slug is included: `name__sheet-sales.summary.md`
  - On collision, a numeric suffix is appended: `name__2.summary.md`

Examples

```bash
# Analyze a CSV (auto-detects comma/semicolon/tab/pipe + locale) and write a summary
smushmux analyze ./data/hops.csv --output hops_summary.md

# TSV with European number format, grouping and correlations
smushmux analyze ./data/sales.tsv \
  --delimiter tab --decimal comma --thousands '.' \
  --group-by region,category --correlations --corr-per-group

# XLSX picking a specific worksheet by name
smushmux analyze ./data/observations.xlsx --sheet-name "Aug 2024"
```

Using instruction templates

- You can steer the AI’s interpretation of the dataset summary by including an instruction markdown file in your project. A ready‑made template lives at `docs/templates/dataset-analysis.md`.
- Two common options:
  - Add it as a project document so it’s merged into the prompt context:
    - `smushmux add -p myproj docs/templates/dataset-analysis.md --desc "Analysis Instructions"`
  - Or set project instructions to the file’s contents (single source of truth):
    - `smushmux instruct -p myproj "$(cat docs/templates/dataset-analysis.md)"`
- Typical flow with a CSV:
  - `smushmux analyze ./data/hops.csv -p myproj --desc "Dataset summary"`
  - `smushmux add -p myproj docs/templates/dataset-analysis.md --desc "Analysis Instructions"`
  - `smushmux generate -p myproj --dry-run --print-prompt` (inspect), then run with your model.

## Examples

See `docs/examples/` for end-to-end guides:

- Quickstart: `docs/examples/quickstart.md`
- Data analysis: `docs/examples/analysis-csv-to-report.md`
- XLSX analysis: `docs/examples/analysis-xlsx-to-report.md`
- Dry-run & tokens: `docs/examples/dry-run-and-tokens.md`
- Model catalog & pricing: `docs/examples/model-catalog.md`
- Output files & formats: `docs/examples/output-and-format.md`
- Recipes (common flows): `docs/examples/recipes.md`
- **Evidence Mode (audit trail)**: `docs/evidence-mode.md`
- Task templates: `docs/templates/` (e.g., `concise-summary.md`)
  - Data analysis: `docs/templates/dataset-analysis.md`

### Retrieval (Lightweight RAG)

SmushMux can augment prompts with retrieved context from your documents.

- Build-and-retrieve in one command:
  ```bash
  # OpenRouter embeddings (default)
  smushmux generate -p myproj --retrieval --embed-model openai/text-embedding-3-small --top-k 6 --min-score 0.2

  # Ollama embeddings
  smushmux config set embedding_provider ollama
  smushmux generate -p myproj --retrieval --embed-model nomic-embed-text --top-k 6 --min-score 0.2
  ```
  This embeds your docs (if not already indexed), searches for the most relevant chunks based on your instructions, and injects them into the prompt.

- Configure defaults in `~/.smushmux/config.yaml`:
  ```yaml
  default_provider: "openrouter"    # or "ollama" for local generation
  embedding_provider: "openrouter"  # or "ollama" for local embeddings
  embedding_model: "openai/text-embedding-3-small"
  retrieval_top_k: 6
  retrieval_min_score: 0.2
  retrieval_include: []         # optional glob patterns (match doc names)
  retrieval_exclude: []         # optional glob patterns to exclude
  retrieval_max_chunks_per_doc: 0  # cap per-doc chunks (0 = no cap)
  ```

- Notes:
  - Index is stored under the project directory as `index.json`.
  - `--reindex` forces rebuilding the index.
  - For OpenRouter embeddings, ensure `OPENROUTER_API_KEY` is set.

## Architecture

- Architecture overview: `ARCHITECTURE.md`
- API surface details: `docs/api.md`

## OpenRouter Setup

1) Create an OpenRouter account and get an API key
2) Export the key

```bash
export OPENROUTER_API_KEY=your_key
```

3) Choose a model (e.g., `openai/gpt-4o-mini`) and run `smushmux generate`.

See `docs/api.md` for request/response details.

## Advanced flags and model catalog

- `--print-prompt`: prints the prompt even for real runs.
- `--prompt-limit N`: truncates the built prompt to N tokens before sending.
- `--timeout-sec N`: sets the request timeout (default 180 seconds).
- `--budget-limit USD`: fails early if estimated max cost (prompt + max-tokens) exceeds the budget.
- `--quiet`: suppresses non-essential console output.
- `--json`: emit response as JSON to stdout.
- `--explain`: emit a human-readable Evidence Report that summarizes run inputs.
- `--json` + `--explain`: supported. When `--json` is set, JSON is written to stdout and the Evidence Report is written to stderr so stdout remains machine-parseable.

### Models catalog

SmushMux ships with a small embedded catalog with approximate context and pricing to provide UX warnings and estimates. You can inspect and override it:

```bash
# Show current catalog
smushmux models show

# Replace catalog from JSON file
smushmux models sync --file ./models.json

# Merge entries from JSON without removing existing ones
smushmux models sync --file ./models.json --merge

# Fetch catalog from URL (optionally save to file)
smushmux models fetch --url https://raw.githubusercontent.com/KaramelBytes/smushmux/main/docs/openrouter-models.json --output models.json --merge

# Apply a built-in preset offline (and optionally save to file)
smushmux models fetch --provider openrouter --merge --output models.json

### Quick preset application during generate

```bash
smushmux generate -p myproj --model-preset openrouter --model openai/gpt-4o-mini --max-tokens 512

# Tiered presets (model selection) with optional provider
# Picks a recommended model if --model is not set
smushmux generate -p myproj --model-preset cheap
smushmux generate -p myproj --model-preset balanced
smushmux generate -p myproj --model-preset high-context
smushmux generate -p myproj --model-preset openrouter:cheap
smushmux generate -p myproj --provider google --model-preset balanced
```
This merges a curated catalog before generation so that warnings (context, cost) reflect the preset.

Observability: on successful responses, the CLI prints a Request ID when available. In --dry-run mode, a deterministic simulated Request ID is printed for traceability.

Providers, Runtimes, and Local-Friendly Models

- Built-in presets now include Gemini and Llama families to better support free and local-friendly scenarios.
- Runtime abstraction: SmushMux uses a runtime interface so backends (OpenRouter, Ollama) can be swapped cleanly.
- Local runtimes (Ollama): set `default_provider: "ollama"` in config to default to local, or pass `--provider ollama` (alias `local`). Ensure Ollama is running (default host `http://127.0.0.1:11434`).
  - Configure in `~/.smushmux/config.yaml`: `ollama_host`, `ollama_timeout_sec`
  - Or env: `SMUSHMUX_OLLAMA_HOST`, `SMUSHMUX_OLLAMA_TIMEOUT_SEC`
  - Examples:
    - `smushmux generate -p demo --model llama3:latest --dry-run`
    - `smushmux generate -p demo --model-preset balanced`
    - `smushmux generate -p demo --model llama3:latest --stream`
```

The JSON format is a simple map of model-name to struct, for example:

```json
{
  "openai/gpt-4o-mini": {
    "Name": "openai/gpt-4o-mini",
    "ContextTokens": 128000,
    "InputPerK": 0.0006,
    "OutputPerK": 0.0024
  }
}
```

## Troubleshooting

- ✗ Error: OPENROUTER_API_KEY is missing
  - Set `OPENROUTER_API_KEY` or add `api_key` in `~/.smushmux/config.yaml`
- Token limit warnings
  - Use `--dry-run` to inspect prompt size; remove or trim large docs
- DOCX parsing issues
  - The parser is a minimal extractor; if parsing fails, convert to `.md` and try again

### Provider catalog presets

You can override provider preset URLs via environment variables until official endpoints are available:

- `SMUSHMUX_OPENROUTER_CATALOG_URL`
- `SMUSHMUX_OPENAI_CATALOG_URL`
- `SMUSHMUX_ANTHROPIC_CATALOG_URL`

These are used by `smushmux models fetch --provider <name>`.

## Limitations

- The CLI performs best with small-to-medium prompt contexts; very large corpora should leverage the retrieval flow and chunking in `internal/retrieval/`.
- DOCX parsing is intentionally minimal and may miss complex formatting. For best results, convert to Markdown.
- Pricing/context metadata in `docs/openrouter-models.json` is approximate and intended for UX warnings, not billing-grade accounting.
- Network calls depend on provider availability; use `--dry-run` and the local `ollama` provider to work offline.

## Notes

This repository is intended as a public, self-contained demonstration of hands-on AI integration:

- End-to-end flow: parsing → analysis → retrieval → generation, with streaming and model presets.
- Clean CLI ergonomics (`cmd/`), modular internals (`internal/`), and clear documentation.
- CI includes build, tests, linting, secret scanning, and CodeQL.

## License

MIT
