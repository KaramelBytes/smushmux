# Evidence Mode

Evidence Mode is a built-in audit trail for `smushmux generate`. When you pass `--explain`, the CLI prints a human-readable **Evidence Report** to stdout before (or instead of) the AI response. The report describes exactly what went into the prompt so you can verify, demo, or archive the inputs without ever making a real API call.

## Why Evidence Mode?

When presenting AI-generated analysis to stakeholders—financial teams, auditors, or clients—it's important to show *what the model saw*. Evidence Mode lets you:

- Confirm which documents were included and how many tokens each contributed
- See which provider and model would be used, and the estimated cost
- Inspect retrieval chunk selections (scores, doc names, previews) when RAG is enabled
- Produce a human-readable workpaper trail using only `--dry-run` (no API key required)

## Typical Workflow

### 1. Initialize a project

```bash
smushmux init myproj -d "Q3 Financial Review"
```

Projects are stored at `~/.smushmux/projects/<name>/` by default. You can override the location via `projects_dir` in `~/.smushmux/config.yaml`.

### 2. Add documents

```bash
smushmux add -p myproj ./reports/q3-financials.md --desc "Q3 financial report"
smushmux add -p myproj ./notes/risk-assessment.txt --desc "Risk notes"
```

Document content is cached inside `project.json` at the time of `add`, so the source files are not re-read at generation time.

### 3. (Optional) Analyze tabular data

If you have CSV/XLSX source data, convert it to a compact Markdown summary first:

```bash
smushmux analyze ./data/transactions.xlsx -p myproj --desc "Transaction summary"
# or in bulk:
smushmux analyze-batch "./data/*.csv" -p myproj
```

Dataset summaries are written to `~/.smushmux/projects/<name>/dataset_summaries/` and added to the project.

### 4. Set instructions

```bash
smushmux instruct -p myproj "Summarize key financial highlights and flag any material risks."
```

### 5. Preview with Evidence Mode (offline, no API key needed)

```bash
smushmux generate -p myproj --dry-run --explain
```

This prints a complete Evidence Report and exits without calling any AI provider.

Sample output:

```
## Evidence Report

### Project
  Name : myproj
  Path : /home/user/.smushmux/projects/myproj

### Provider / Model
  Provider     : (default)
  Model        : openai/gpt-4o-mini
  Max Tokens   : 0

### Prompt Statistics
  Prompt tokens (approx) : 312
  Max tokens requested   : 0
  Prompt limit           : (none)
  Budget limit           : (none)

### Retrieval
  Enabled : no

### Documents Included in Prompt
  1. q3-financials.md — Q3 financial report
     Source : /home/user/reports/q3-financials.md
     Tokens : ~210
  2. risk-assessment.txt — Risk notes
     Source : /home/user/notes/risk-assessment.txt
     Tokens : ~55
```

### 6. Run for real (requires API key)

```bash
export OPENROUTER_API_KEY=your_key_here
smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 512
```

## Key Artifacts

| Location | Description |
|---|---|
| `~/.smushmux/projects/<name>/project.json` | Project metadata, document cache, and instructions |
| `~/.smushmux/projects/<name>/index.json` | Retrieval index (created when `--retrieval` is used) |
| `~/.smushmux/projects/<name>/dataset_summaries/` | Compact Markdown summaries from `analyze`/`analyze-batch` |

## Evidence Mode with Retrieval

When retrieval is enabled, the Evidence Report includes an additional **Retrieval evidence** section listing the top-k chunks that were selected, their scores, and a short preview of each chunk:

```bash
smushmux generate -p myproj --dry-run --explain \
  --retrieval --embed-model openai/text-embedding-3-small --top-k 5
```

Sample output (additional section):

```
## Retrieval evidence

  Top-K     : 5
  Min score : 0.00

  1. [score=0.8712] q3-financials.md (chunk 0)
     Preview: Total revenue for Q3 reached $4.2M, representing a 12% increase year-over-year...
  2. [score=0.7340] risk-assessment.txt (chunk 0)
     Preview: FX exposure: 15% of revenue denominated in EUR; hedging strategy in place...
```

## `--json` and `--explain` Interaction

Do **not** combine `--json` and `--explain` in the same invocation. `--json` emits the AI response as a machine-parseable JSON object to stdout, while `--explain` writes a human-readable report to the same stdout stream. Mixing them produces unstructured output. Use each flag independently:

```bash
# Human-readable evidence report (offline)
smushmux generate -p myproj --dry-run --explain

# Machine-parseable JSON response (requires API key)
smushmux generate -p myproj --json
```

## Offline Demo Checklist

1. `smushmux init demo -d "Demo project"` — creates project directory
2. `smushmux add -p demo <file>` — caches document content locally
3. `smushmux instruct -p demo "..."` — sets the prompt instructions
4. `smushmux generate -p demo --dry-run --explain` — produces Evidence Report with **zero network calls**

No `OPENROUTER_API_KEY` or Ollama installation is required for steps 1–4.
