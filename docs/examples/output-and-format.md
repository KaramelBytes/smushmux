# Output Files and Formats

SmushMux lets you control presentation, persistence, and machine-readable output.

## Save responses to files
```bash
# Markdown (default)
smushmux generate -p myproj --output out.md --format markdown

# Plain text
smushmux generate -p myproj --output out.txt --format text
```

## Emit JSON for automation
```bash
# Structured output to stdout
smushmux generate -p myproj --json | jq .

# Write full response payload to a file
smushmux generate -p myproj --json --output out.json --format json
```

## Control logging and streaming
```bash
# Quiet mode hides non-essential logs
smushmux generate -p myproj --quiet --output out.md

# Real-time deltas (OpenRouter or Ollama runtimes)
smushmux generate -p myproj --stream --model openai/gpt-4o-mini --max-tokens 512
```

## Inspect prompts before sending
```bash
smushmux generate -p myproj --print-prompt --max-tokens 1024
```
