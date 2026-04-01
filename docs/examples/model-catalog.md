# Model Catalog and Pricing

SmushMux keeps a lightweight catalog to warn about context limits and cost. You can inspect or override it at any time.

## Inspect current catalog
```bash
smushmux models show | jq .
```

## Replace or merge from JSON
```bash
smushmux models sync --file ./models.json          # replace
smushmux models sync --file ./models.json --merge  # merge into existing entries
```

## Fetch from URL or provider preset
```bash
# Remote JSON (optional --output to persist locally)
smushmux models fetch --url https://example.com/models.json --merge --output models.json

# Built-in presets (offline)
smushmux models fetch --provider openrouter --merge --output models-openrouter.json
```

## Use presets during generate
```bash
# Merge the OpenRouter preset and pick a cheap model automatically
smushmux generate -p myproj --model-preset cheap

# Combine provider + tier
smushmux generate -p myproj --provider google --model-preset balanced

# Explicit model with preset warnings
smushmux generate -p myproj --model openai/gpt-4o-mini --model-preset openrouter --max-tokens 512
```

## Budget awareness during generate
```bash
smushmux generate -p myproj --model openai/gpt-4o-mini --max-tokens 512 --budget-limit 0.03
# Prints estimated max cost and fails early if the limit would be exceeded
```
