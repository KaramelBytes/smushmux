# Analyze Multiple Datasets with Progress

Use `analyze-batch` to summarize many CSV/TSV/XLSX files with a single command. This prints progress as each file is processed and can attach summaries to a project.

**Mixed inputs**: Tabular files (`.csv`, `.tsv`, `.xlsx`) are analyzed into summaries. Non-tabular files (`.yaml`, `.md`, `.txt`, `.docx`) are added as regular documents when `-p` is provided; otherwise skipped with a warning.

## Examples

- Process a folder of datasets with progress

```bash
smushmux analyze-batch "data/*.csv"
```

- Attach all summaries to a project (and suppress sample tables)

```bash
smushmux analyze-batch "data/*.xlsx" \
  -p brewlab --desc "Batch dataset summaries" \
  --sample-rows-project 0
```

- Select XLSX sheet and set CSV/locale options

```bash
smushmux analyze-batch data/*.xlsx \
  --sheet-name "Aug 2024" \
  --delimiter ',' --decimal dot --thousands ,
```

## Behavior

- Shows progress: `[N/Total] Processing <file>...` (use `--quiet` to suppress)
- Mirrors `analyze` flags (grouping, correlations, outliers, locale)
- **Tabular files** (`.csv`, `.tsv`, `.xlsx`):
  - Analyzed into summaries and attached to `dataset_summaries/` when `-p` is provided
  - Filenames are disambiguated:
    - With `--sheet-name`, sheet slug is added: `name__sheet-sales.summary.md`
    - On collision, an increment is appended: `name__2.summary.md`
  - Use `--sample-rows-project` to override sample rows for all outputs (set `0` to disable sample tables)
- **Non-tabular files** (`.yaml`, `.md`, `.txt`, `.docx`):
  - Added as regular documents to the project when `-p` is provided
  - Skipped with a warning if no project is specified
