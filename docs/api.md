# OpenRouter Integration (SmushMux)

This document describes how SmushMux interacts with the OpenRouter API for chat completions.

## Endpoint (OpenRouter)

- URL: `POST https://openrouter.ai/api/v1/chat/completions`
- Headers:
  - `Authorization: Bearer <OPENROUTER_API_KEY>`
  - `Content-Type: application/json`
  - `HTTP-Referer: https://github.com/KaramelBytes/smushmux`
  - `X-Title: SmushMux CLI`
  - On responses, SmushMux captures common request ID headers (e.g., `X-Request-Id`, `OpenAI-Request-ID`) and prints them on success for observability.

## Request Body (MVP)

```json
{
  "model": "openai/gpt-4o-mini",
  "messages": [
    { "role": "user", "content": "[INSTRUCTIONS]... [REFERENCE DOCUMENTS]... [TASK]..." }
  ],
  "max_tokens": 1024,
  "temperature": 0.7
}
```

- `model`: Provider/model identifier as listed by OpenRouter (e.g., `openai/gpt-4o-mini`).
- `messages`: Single-message user prompt containing instructions, documents, and task.
- `max_tokens`: Target maximum response tokens.
- `temperature`: Sampling temperature.

## Response Body (Simplified)

```json
{
  "id": "resp_...",
  "choices": [
    {
      "message": { "role": "assistant", "content": "..." }
    }
  ],
  "usage": {
    "prompt_tokens": 1234,
    "completion_tokens": 456,
    "total_tokens": 1690
  }
}
```

SmushMux prints the first choice's message content.
If a request ID is present in response headers, SmushMux prints it for traceability.

## Errors

- Non-2xx status codes are returned with a short message containing `status` and parsed body (if possible).
- Common issues:
  - Missing or invalid API key
  - Unsupported model name
  - Token/context limit exceeded

## Token and Context Notes

SmushMux estimates prompt tokens locally and prints warnings. Context limits vary by model; verify limits on OpenRouter model docs. Use `--dry-run` to preview prompt and token breakdown without performing an API call.

## Security

- API key is never logged.
- Read from environment (`OPENROUTER_API_KEY`) or config (`~/.smushmux/config.yaml`).

## Local Runtime (Ollama)

- Select with `--provider ollama` (alias: `local`) or set `default_provider: ollama` in config.
- Host defaults to `http://127.0.0.1:11434`; configure with `ollama_host` or `SMUSHMUX_OLLAMA_HOST`.
- API: `POST /api/chat` with `stream=false`.
- On success, SmushMux prints a local correlation token (simulated Request ID).
 - Streaming: use `--stream` to enable incremental output. Supported for Ollama and OpenRouter.

## Runtime Abstraction

SmushMux defines a small runtime interface so multiple backends can implement the same `Generate` surface. The CLI selects the runtime via `--provider` and applies model presets independently of the runtime choice.

## Model Catalog and Presets

- Inspect current catalog: `smushmux models show | jq .`
- Apply a built-in provider preset without network: `smushmux models fetch --provider openrouter --merge`
- During generation you can apply presets inline:
  - Provider catalog: `smushmux generate -p myproj --model-preset openrouter`
  - Tiered selection (chooses a model if `--model` not set): `smushmux generate -p myproj --model-preset cheap|balanced|high-context`
  - Explicit provider guidance: `smushmux generate -p myproj --provider google --model-preset balanced`
  - Combined: `smushmux generate -p myproj --model-preset openrouter:cheap`
  - Presets merge a curated catalog before generation so pricing/context warnings reflect it.
