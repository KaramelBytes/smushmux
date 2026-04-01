# Development History

> **Note:** SmushMux was developed throughout late 2025 under a different project name; a naming conflict prompted a rebrand and fresh repository. The timeline and changes below reflect actual development history from the original local work.

## v1 Stabilization (April 2026)

This update formally freezes the feature set of the legacy single-shot architecture (SmushMux v1), marking it complete and setting limits to optimize local LLM usage on memory-constrained hardware (e.g., 8GB RAM Macs) before the transition to an agentic vNext.

### 🎉 Added
- **OOM Prevention**: `max_context_cap` configuration property added to formally override and clamp model context limits that are too large for the host machine.
- **Tabular Boundary Enforcement**: Parser defaults now rigidly enforce hard-limits (e.g. max 50 columns, max 1000 rows) out of the box when dealing with `xlsx` and `csv` files inside `internal/analysis/table.go` arrays to prevent token explosion.

### 📚 Documentation
- Updated `README.md` and `ARCHITECTURE.md` with explicit "Cutoff Version" notifications discussing constraints for memory-limited Macs.
- Marked remaining items in `docs/testing-roadmap.md` as skipped/archived, signifying the stability of the core prior to vNext's architectural shift.

### 🔧 Changed
- Table summaries automatically drop excess columns and fast-continue through rows exceeding bounds instead of attempting to consume processing time or expanding the token payload further.

## Evidence Mode & CI Overhaul (March 2026)

### 🎉 Added
- **Evidence Mode (`--explain`)**: `smushmux generate` can now print a human-readable Evidence Report summarizing run inputs, token estimates, and prompt composition details.
- **Retrieval Evidence in Explain Output**: When `--retrieval` is enabled and results are present, explain output includes a retrieval evidence section with ranked chunks, scores, and previews.
- **Offline Explain Coverage**: Added fixture-backed integration coverage for `--dry-run --explain` flows without requiring network/API calls.

### 📚 Documentation
- Added `docs/evidence-mode.md` with usage examples, retrieval behavior, and `--json`/`--explain` guidance.
- Updated README examples index to include the Evidence Mode guide.
- Updated quickstart example flow to include an explicit offline `--dry-run --explain` step.

### 🔧 Changed
- CI lint workflow now pins `golangci-lint` and runs a stable, low-noise linter set (`errcheck`, `govet`, `ineffassign`, `staticcheck`).
- CodeQL workflow now skips private repos by default and supports explicit opt-in via repository variable `ENABLE_CODEQL=true`.
- Added a lightweight `govulncheck` security workflow for dependency and code vulnerability checks.
- Secret scan now runs `gitleaks` CLI directly in CI (no code-scanning upload dependency), reducing private-repo permission failures.
- Security workflow now runs `govulncheck` via `go run ...@version` to avoid PATH-related failures on runners.
- Fixed gitleaks installer module path to `github.com/zricethezav/gitleaks/v8`, resolving CI install failures.
- Security workflow now uses a patched Go toolchain (`1.25.x`) so govulncheck is not blocked by known vulnerabilities in older Go stdlib versions.

### 📦 Dependencies
- Bumped `github.com/go-viper/mapstructure/v2` from `v2.2.1` to `v2.4.0`.

### 🧪 Testing
- Added tests covering explain report rendering and retrieval evidence sections.
- Added integration tests validating evidence mode behavior with fixture projects.
- Added backlog note to expand offline test coverage for packages that currently report no test files (for example, root package and internal/config).
- Added `docs/testing-roadmap.md` with prioritized offline test coverage targets and effort estimates.
- Added offline unit tests for `internal/config` (`Load`/`Save` defaults, env precedence, and file path behavior).
- Added offline unit tests for `internal/utils/files.go` (`EnsureProjectDir`, `SafeWriteFile`, `PrettyJSON`, and `FindProjectRoot`).
- Added offline command tests for `cmd/config.go` and `cmd/models.go` (validation, normalization, and preset-fetch behavior).
- Added parser edge-case tests for markdown normalization, DOCX parse failure/success paths, and TXT round-trip behavior.
- Added lifecycle command tests for `init`, `add`, `list`, and `project set-model` validation/error/set-clear paths.
- Added XLSX parser guard-path test for missing input files.
- Added command output behavior tests for `config show`, `models show`, and project listing output paths.
- Added root command tests for startup config override behavior and local catalog fetch/apply handling.
- Added broad offline `internal/ai` tests for catalog operations, runtime defaults/registry, API error classification helpers, retry helper behavior, Ollama embeddings client paths, and malformed stream/embedding payload handling.
- Added a lightweight CI coverage gate enforcing minimum package coverage for `cmd` (>=55%) and `internal/ai` (>=60%).
- Added optional provider-specific smoke-test workflow (`.github/workflows/smoke-test.yml`) with manual dispatch trigger for Ollama and OpenRouter free-tier testing.
- Added smoke tests for Ollama basic completion (`TestSmokeOllamaBasicCompletion`) and OpenRouter free-tier classification (`TestSmokeOpenRouterFreeTier`) in `internal/ai/smoke_test_providers_test.go`.
- Smoke tests are skipped by default and only execute when explicitly enabled via env vars or GitHub Actions workflow dispatch; see `docs/testing-roadmap.md#phase-7` for usage details.

## Batch Analysis & Memory Safety Improvements (Mid-October 2025)

### 🎉 Added
- **Batch Analysis**: New `analyze-batch` command processes multiple files with `[N/Total]` progress
- **Mixed-Input Batch**: Supports `.csv`, `.tsv`, `.xlsx` (analyzed) + `.yaml`, `.md`, `.txt`, `.docx` (added as docs)
- **Project-Level Sample Control**: `--sample-rows-project` flag to override samples in all summaries (set `0` to disable)
- **Memory Safety**: Hard limits prevent OOM (200k tokens, 20 summaries per project)
- **Context Validation**: Blocks oversized prompts for local LLMs with actionable error messages
- **Timeout Configuration**: `--timeout-sec` flag for generation requests (default 180s)
- **TSV Auto-Delimiter**: Automatically sets tab delimiter for `.tsv` files

### 🐛 Fixed
- **CRITICAL**: XLSX parser returning 0 columns due to absolute relationship paths in ZIP archives
- Unbounded memory accumulation with multiple large files (9.3GB → <2GB peak)
- Duplicate document detection (no more silent overwrites)
- Memory leaks in outlier computation
- Context window overflow causing silent truncation in Ollama
- RAG chunker producing oversized chunks exceeding token limits
- Prompt instruction duplication (40% token reduction)
- Dataset summary basename collisions with disambiguation logic
- Invalid `--sheet-name` silently falling back to first sheet

### ⚡ Performance
- Reduced memory usage by 78% for multi-file projects
- Batched embedding prevents API timeout failures (100 chunks/batch)
- 40% reduction in prompt tokens via deduplication
- Immediate memory release after outlier computation

### 💥 Breaking Changes
- Context overflow now **blocks** execution for Ollama (was warning-only)
- Duplicate files now **error** instead of silently overwriting
- Invalid `--sheet-name` now errors with available sheet list
- Projects enforce maximum 200k token limit (hard cap at 200k)
- Maximum 20 dataset summaries per project (prevents context bloat)

### 📚 Documentation
- Added docs/examples/analyze-batch.md with batch processing examples
- Updated README with mixed-input batch behavior
- Added XLSX parser fix details and regression test
- Updated quickstart with batch analysis tips

### 🧪 Testing
- Added regression test for XLSX relationship path normalization
- Added integration test for batch analysis with sample suppression
- Memory profiling tests ensure <2GB peak for 10x100k-row files
- Race detector clean across all packages

## Initial MVP (Early October 2025)

### Added
- Initial release
- Basic project management (`init`, `add`, `list`)
- CSV/TSV/XLSX analysis with schema inference
- OpenRouter, Ollama, and major provider support
- RAG with embedding indexes
- Model catalog management
