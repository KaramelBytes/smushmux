# CSV/TSV/XLSX Analysis to Project Report

Convert a dataset into a compact summary, attach instructions, and generate an AI report.

## Goal
- Summarize a dataset (CSV/TSV/XLSX) into Markdown with schema inference, stats, optional grouping/correlations/outliers.
- Attach the summary to a project, add an instruction template, optionally enable retrieval, and run generation.

## Prerequisites
- SmushMux built or installed (`smushmux --help`).
- Optional: `OPENROUTER_API_KEY` if you plan to do a real run (non-dry-run).

## 1) Prepare a small CSV (example)

```bash
mkdir -p data
cat > data/hops.csv <<'CSV'
date,plot,alpha_acids,moisture
2024-08-10,A1,12.5%,74
2024-08-12,A1,11.8%,71
2024-08-15,B3,10.2%,68
CSV
```

Notes
- Delimiters: comma, semicolon, tab, and pipe are auto-detected (override with `--delimiter`).
- TSVs and XLSX files are also supported (`.tsv`, `.xlsx`).
- For XLSX, pick a sheet via `--sheet-name` or `--sheet-index`.

## 2) Create a project

```bash
smushmux init brewlab -d "Hop harvest analysis"
```

## 3) Analyze the dataset and attach to the project

```bash
# CSV: auto-detect delimiter & locale, then write the summary into the project
smushmux analyze ./data/hops.csv -p brewlab --desc "Hops dataset summary"

# Optional: also write a copy to a standalone file
smushmux analyze ./data/hops.csv --output hops_summary.md
```

Common flags
- `--group-by plot` to add per-group metrics.
- `--correlations` to compute Pearson correlations among numeric columns.
- `--corr-per-group` to compute correlations per group (slower).
- Locale: `--delimiter ','|'tab'|';'`, `--decimal '.'|'comma'`, `--thousands ','|'.'|'space'`.
- XLSX: `--sheet-name <name>` or `--sheet-index <n>`.

## 4) Add explicit analysis instructions (template)

```bash
# Add a ready-made template to guide interpretation of the dataset summary
smushmux add -p brewlab docs/templates/dataset-analysis.md --desc "Analysis Instructions"

# Alternatively, set the project instructions from the file content
# smushmux instruct -p brewlab "$(cat docs/templates/dataset-analysis.md)"
```

## 5) Inspect prompt and generate

```bash
# Dry-run to inspect prompt composition and size (no API call)
smushmux generate -p brewlab --dry-run --print-prompt

# Real run (requires OPENROUTER_API_KEY)
export OPENROUTER_API_KEY=your_key
smushmux generate -p brewlab --model openai/gpt-4o-mini --max-tokens 600
```

## 6) (Optional) Retrieval for context-aware answers

```bash
# Build/refresh embedding index, retrieve top chunks, and include them in the prompt
smushmux generate -p brewlab --retrieval --embed-model openai/text-embedding-3-small --top-k 6 --min-score 0.2 --dry-run

# Local embeddings via Ollama
smushmux generate -p brewlab --retrieval --embed-provider ollama --embed-model nomic-embed-text --model llama3:latest --dry-run
```

## What to expect in the dataset summary
- `[DATASET SUMMARY]` with row counts and multi-column sample rows.
- `[SCHEMA]` with per-column type inference (numeric, datetime, categorical, text) and units.
- Optional `[GROUP-BY SUMMARY]` sections if you used `--group-by`.
- Optional `[CORRELATIONS]` with a matrix and/or top pairs when `--correlations` is set.
- Outlier counts (robust Z via MAD) when outliers are enabled (default true).

## Tips
- Adding the summary instead of raw tables keeps prompts concise and token-efficient.
- Use `--sample-rows` and `--max-rows` to tune summary size vs. fidelity.
- Start with `--correlations` only when you need it; pairwise stats increase compute.
- Retrieval is most helpful when instructions call for thin slices of the summary (e.g., “focus on plots with low alpha acids”).
