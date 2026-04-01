# Architecture Overview

> 🎩 **Important Cut-Off Context (April 2026):** 
> The architecture outlined below represents the "single-shot," statistical-summarization design pattern which functioned excellently for compiling experimental plant genetics trends, but began breaking down due to small memory models and rigorous financial reporting requirements. Instead of breaking its stable core, the codebase was frozen here. Key tradeoffs made included capping document tables intentionally limits (max 50 cols / ~1000 rows) and enforcing configurable max context caps for memory reduction on 8GB machines. These deliberate functional ceilings establish a stepping-off point to an "agentic tool-use" architecture in the upcoming vNext.

SmushMux is a Go CLI that merges documents into a single, AI‑ready prompt and routes the request to either OpenRouter or a local Ollama runtime. The implementation is intentionally modular so that new formats, providers, or retrieval strategies can be added without rewiring the CLI surface.

## Layered Design
- **CLI (`cmd/`)**: Cobra commands own flag parsing, config loading, orchestration, and human-friendly output. Integration tests exercise the primary flows end to end.
- **Domain (`internal/project`)**: Manages project persistence (`project.json`), document registry, prompt construction, and per-project overrides. Uses helpers in `internal/utils` for atomic file writes and token estimation.
- **Parsing & Analysis (`internal/parser`, `internal/analysis`)**: `ParseFile` dispatches to format-specific parsers. Text and Markdown are read directly, DOCX is unzipped and cleaned, and tabular formats (CSV/TSV/XLSX) funnel through the analysis package to produce concise Markdown summaries.
- **Retrieval (`internal/retrieval`)**: Builds and maintains an embedding index (`index.json`) per project. Supports configurable chunking, include/exclude filters, and cosine similarity search, with embeddings sourced from OpenRouter or Ollama depending on configuration.
- **AI Runtimes (`internal/ai`)**: Provides a runtime registry plus concrete clients for OpenRouter and Ollama. Handles retries, rate limiting, streaming, embeddings, and a shared model catalog with pricing/context metadata.
- **Configuration (`internal/config`)**: Viper-backed loader that merges defaults, config files, environment variables, and CLI flags. Exposes settings for providers, retry policy, retrieval defaults, and Ollama tuning.

## Data & Storage
- Projects live under `~/.smushmux/projects/<name>/` (configurable via `projects_dir`). The directory contains `project.json`, optional `dataset_summaries/` entries, and `index.json` when retrieval is enabled.
- `project.json` captures metadata: documents, descriptions, instructions, per-project model overrides, and timestamps.
- The in-memory model catalog seeds estimates for context/token warnings. Users can replace or merge catalogs via `smushmux models` commands or auto-sync on startup.

## Generate Flow (Happy Path)
1. Resolve the target project and load `project.json`.
2. Build the base prompt and approximate token counts.
3. If retrieval is requested, refresh/consult the embedding index, fetch the top matching chunks, and splice them into the prompt.
4. Enforce optional prompt limit or budget guard before dispatch.
5. Select the runtime (OpenRouter or Ollama), apply model/preset decisions, and issue either a regular or streaming request.
6. Print structured output, optionally save to file/JSON, and surface request IDs or usage metadata when available.

## Error Handling
- Runtime clients classify common failures (authentication, rate limit, model not found, quota exceeded, unreachable host) and the CLI maps them to actionable feedback.
- Rate limiting honours `Retry-After` headers. Network hiccups leverage exponential backoff with jitter; sandboxed environments skip network-dependent tests gracefully.

## Configuration Precedence
CLI flags → Environment (`SMUSHMUX_*`, `OPENROUTER_API_KEY`) → Config file (`~/.smushmux/config.yaml`) → Built-in defaults.

## Extension Points
- **Parsers**: register new types in `internal/parser` and delegate heavy lifting to a helper package if needed.
- **Providers**: implement the runtime interface, register it in `internal/ai/runtime_registry.go`, and expose presets through `cmd/models.go` if catalog data is available.
- **Retrieval**: tweak chunking strategy or add alternative similarity scoring inside `internal/retrieval`.
- **Output**: extend `cmd/generate_helpers.go` to add new formats or destinations (e.g., append mode, HTML export).
