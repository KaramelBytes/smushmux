# Using SmushMux with Ollama (Local Runtime)

This guide shows how to generate locally using an Ollama runtime.

## Prerequisites
- Install and run Ollama: https://ollama.com
- Ensure the model you want is available, e.g. `ollama pull llama3:latest`

## Examples

- Dry run (no API call) with local provider preset:
```
smushmux generate -p myproj --provider ollama --model llama3:latest --dry-run --print-prompt
```

- Streaming output from a local model:
```
smushmux generate -p myproj --provider ollama --model llama3:latest --stream
```

- Choose by tier with a preset (if you didn’t set `--model` explicitly):
```
smushmux generate -p myproj --provider ollama --model-preset balanced
```

- Retrieval using local embeddings (builds/refreshes `index.json`):
```
smushmux generate -p myproj --provider ollama \
  --retrieval --embed-provider ollama --embed-model nomic-embed-text \
  --model llama3:latest --top-k 6 --min-score 0.2
```

- Configure host/timeout via env or config:
```
export SMUSHMUX_OLLAMA_HOST="http://127.0.0.1:11434"
export SMUSHMUX_OLLAMA_TIMEOUT_SEC=120
```
Or in `~/.smushmux/config.yaml`:
```
ollama_host: "http://127.0.0.1:11434"
ollama_timeout_sec: 120
```

If a model is missing, SmushMux will suggest pulling it, e.g.: `ollama pull llama3:latest`.

Tip: To default to local runs, set in `~/.smushmux/config.yaml`:
```
default_provider: ollama
```

For retrieval defaults you can also set:
```
embedding_provider: ollama
embedding_model: nomic-embed-text
retrieval_top_k: 6
retrieval_min_score: 0.2
```
