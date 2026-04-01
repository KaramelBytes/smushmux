#!/usr/bin/env bash
set -euo pipefail

# Smoke test for the CLI in a clean HOME, without modifying user state.
# - Always runs init/add/instruct and a dry-run generate
# - If OPENROUTER_API_KEY is set, runs a short real generate
# - If Ollama is reachable, tries a short local dry-run (and streaming if requested)

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
export HOME=$(mktemp -d)
PROJECT=smoke

echo "[i] Using temporary HOME at: $HOME"
echo "[i] Root: $ROOT_DIR"

cd "$ROOT_DIR"

DOC="$HOME/doc.md"
cat > "$DOC" << 'EOF'
# Smoke Test Document

This is a short document for smoke testing the CLI.
EOF

echo "[1/6] init project"
go run . init "$PROJECT" -d "Smoke test project" >/dev/null

echo "[2/6] add document"
go run . add -p "$PROJECT" "$DOC" --desc "sample" >/dev/null

echo "[3/6] set instructions"
go run . instruct -p "$PROJECT" "Summarize the content in one paragraph." >/dev/null

echo "[4/6] dry-run generate (prompt preview; offline)"
go run . generate -p "$PROJECT" --dry-run --prompt-limit 2000 --print-prompt >/dev/null
echo "    ✓ dry-run ok"

echo "[5/6] OpenRouter real generate check"
if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "    OpenRouter real run skipped (OPENROUTER_API_KEY not set)"
else
  if [[ "${SMOKE_TRY_OPENROUTER_RUN:-}" = "1" ]]; then
    tier="${SMOKE_OPENROUTER_TIER:-cheap}"
    provider="${SMOKE_OPENROUTER_PROVIDER:-}"
    model="${SMOKE_OPENROUTER_MODEL:-}"
    budget="${SMOKE_OPENROUTER_BUDGET:-0.02}"
    args=(generate -p "$PROJECT" --max-tokens 64 --budget-limit "$budget" --quiet)
    if [[ -n "$provider" ]]; then args+=(--provider "$provider"); fi
    if [[ -n "$model" ]]; then
      args+=(--model "$model")
      echo "    Trying OpenRouter run (model=$model, budget=$budget)"
    else
      args+=(--model-preset "$tier")
      echo "    Trying OpenRouter run (tier=$tier${provider:+, provider=$provider}, budget=$budget)"
    fi
    set +e
    go run . "${args[@]}"
    rc=$?
    set -e
    if [[ $rc -eq 0 ]]; then
      echo "    ✓ OpenRouter generation ok"
    else
      echo "    ⚠ OpenRouter generation failed (rc=$rc). Adjust SMOKE_OPENROUTER_MODEL/PROVIDER/TIER or SMOKE_OPENROUTER_BUDGET."
    fi
  else
    echo "    OpenRouter real run skipped (set SMOKE_TRY_OPENROUTER_RUN=1)"
  fi
fi

echo "[6/6] Local (Ollama) dry-run check"
OLLAMA_HOST="${SMUSHMUX_OLLAMA_HOST:-http://127.0.0.1:11434}"
if command -v curl >/dev/null 2>&1; then
  resp=$(curl -fsS "$OLLAMA_HOST/api/tags" 2>/dev/null || true)
  if echo "$resp" | grep -q '"models"'; then
    echo "    Ollama reachable at $OLLAMA_HOST"
    # Gather available model names
    if command -v jq >/dev/null 2>&1; then
      mapfile -t MODELS < <(echo "$resp" | jq -r '.models[].name')
    else
      mapfile -t MODELS < <(echo "$resp" | grep -o '"name":"[^"]*"' | sed -E 's/"name":"(.*)"/\1/')
    fi
    # Choose a reasonable chat-capable default if not overridden
    CHOSEN="${SMOKE_OLLAMA_MODEL:-}"
    choose_model() {
      for m in "${MODELS[@]}"; do
        case "$m" in
          llama3:*|llama3.*)
            case "$m" in *instruct* ) echo "$m"; return 0;; esac ;;
          mistral:*)
            case "$m" in *instruct* ) echo "$m"; return 0;; esac ;;
          phi3:*) echo "$m"; return 0 ;;
          tinyllama:*) echo "$m"; return 0 ;;
          gemma2:*) echo "$m"; return 0 ;;
        esac
      done
      # fallback to first model
      if [[ ${#MODELS[@]} -gt 0 ]]; then echo "${MODELS[0]}"; return 0; fi
      return 1
    }
    if [[ -z "$CHOSEN" ]]; then
      CHOSEN=$(choose_model || true)
    fi
    if [[ -z "$CHOSEN" ]]; then
      echo "    ⚠ No models found in Ollama tag list; skipping local test"
    else
      echo "    Using model: $CHOSEN"
      echo "    (dry-run only; no API call to Ollama is made)"
      set +e
      go run . generate -p "$PROJECT" --provider ollama --model "$CHOSEN" --dry-run --print-prompt >/dev/null
      rc=$?
      set -e
      if [[ $rc -eq 0 ]]; then
        echo "    ✓ Ollama dry-run ok ($CHOSEN)"
      else
        echo "    ⚠ Ollama dry-run failed (rc=$rc). Try: 'ollama pull $CHOSEN' or set SMOKE_OLLAMA_MODEL"
      fi
      if [[ "${SMOKE_TRY_OLLAMA_RUN:-}" = "1" ]]; then
        echo "    Trying a short real run against Ollama (may fail if model missing)"
        set +e
        go run . generate -p "$PROJECT" --provider ollama --model "$CHOSEN" --max-tokens 16 --quiet
        rc=$?
        set -e
        if [[ $rc -eq 0 ]]; then
          echo "    ✓ Ollama real run ok ($CHOSEN)"
        else
          echo "    ⚠ Ollama real run failed (rc=$rc). Try: 'ollama pull $CHOSEN' or set SMOKE_OLLAMA_MODEL"
        fi
      fi
    fi
  else
    echo "    Ollama not reachable (no valid JSON from /api/tags); skipping local test"
  fi
else
  echo "    curl not available; skipping Ollama check"
fi

echo "[✓] Smoke test completed"
