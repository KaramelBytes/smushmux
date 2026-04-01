#!/usr/bin/env bash
set -euo pipefail

# Gitleaks pre-commit/CI helper
# Scans staged changes by default; pass --all to scan the full repo.

if [[ "${SKIP_GITLEAKS:-}" == "1" ]]; then
  echo "[secret-scan] Skipping gitleaks (SKIP_GITLEAKS=1)"
  exit 0
fi

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "[secret-scan] gitleaks not found in PATH." >&2
  echo "Install: https://github.com/gitleaks/gitleaks#installation or set SKIP_GITLEAKS=1 to bypass." >&2
  exit 2
fi

mode="staged"
if [[ "${1:-}" == "--all" ]]; then
  mode="all"
fi

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

if [[ "$mode" == "staged" ]]; then
  if [[ -z "$(git diff --cached --name-only --diff-filter=ACMR)" ]]; then
    echo "[secret-scan] No staged changes to scan."
    exit 0
  fi
  echo "[secret-scan] Running gitleaks on staged changes..."
  gitleaks detect --staged --redact --no-banner --exit-code 1
else
  echo "[secret-scan] Running gitleaks on repository..."
  gitleaks detect --redact --no-banner --exit-code 1
fi

