# XLSX Analysis with Sheet Selection

Analyze specific worksheets from an `.xlsx` workbook, attach instruction templates, and generate an AI report.

## Goal
- Summarize selected worksheet(s) from an XLSX into Markdown.
- Attach summaries and instructions to a project, optionally enable retrieval, and produce a final report.

## Prerequisites
- SmushMux built or installed (`smushmux --help`).
- An XLSX file at `./data/observations.xlsx` with at least two sheets (e.g., "Aug 2024" and "Sep 2024").
  - Columns example: `date`, `site`, `yield_kg`, `moisture_%`.
  - Create the workbook in any spreadsheet tool and save as `.xlsx`.

## 1) Create a project

```bash
smushmux init fieldstudy -d "Monthly observations"
```

## 2) Analyze a specific sheet by name

```bash
# Analyze the worksheet named "Aug 2024" and attach the summary to the project
smushmux analyze ./data/observations.xlsx \
  --sheet-name "Aug 2024" \
  -p fieldstudy --desc "Observations (Aug 2024)"

# Optionally write a standalone copy
smushmux analyze ./data/observations.xlsx --sheet-name "Aug 2024" --output aug_summary.md
```

## 3) Analyze another sheet by index

```bash
# If the second sheet holds September, index is 2 (1-based)
smushmux analyze ./data/observations.xlsx \
  --sheet-index 2 \
  -p fieldstudy --desc "Observations (Sep 2024)"
```

Common additions
- Use `--group-by site` to get per-site metrics.
- Add `--correlations` or `--corr-per-group` when you need relationships between numeric fields.
- `--sample-rows` and `--max-rows` control how much data is surfaced in the summary.

## 4) Add explicit analysis instructions (template)

```bash
# Template guides the model’s interpretation for dataset reviews
smushmux add -p fieldstudy docs/templates/dataset-analysis.md --desc "Analysis Instructions"

# Or replace project instructions entirely
# smushmux instruct -p fieldstudy "$(cat docs/templates/dataset-analysis.md)"
```

## 5) Inspect the prompt and generate

```bash
# Dry-run for inspection
smushmux generate -p fieldstudy --dry-run --print-prompt

# Real run (requires OPENROUTER_API_KEY)
export OPENROUTER_API_KEY=your_key
smushmux generate -p fieldstudy --model openai/gpt-4o-mini --max-tokens 700
```

## 6) (Optional) Retrieval for large workbooks

```bash
# Let SmushMux pull the most relevant chunks from your summaries
smushmux generate -p fieldstudy --retrieval --embed-model openai/text-embedding-3-small --top-k 8 --min-score 0.2 --dry-run

# Local embeddings via Ollama (requires an embedding model, e.g., nomic-embed-text)
smushmux generate -p fieldstudy --retrieval --embed-provider ollama --embed-model nomic-embed-text --model llama3:latest --dry-run
```

## Resulting summary highlights
- `[DATASET SUMMARY]` with row/column counts, sample records.
- `[SCHEMA]` with type detection, units, null counts, outliers.
- Optional grouping sections and correlation matrices based on flags.

## Tips
- Keep each worksheet summary focused—break apart large sheets for better retrieval relevance.
- Retrieval is especially useful when you have multiple summaries attached to the same project.
- Use `smushmux list --docs -p fieldstudy` to confirm summaries and templates are attached as expected.
