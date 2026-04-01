#!/usr/bin/env bash
set -euo pipefail

# Symlink our repo-managed pre-commit hook into .git/hooks
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
hook_src="$repo_root/scripts/hooks/pre-commit"
hook_dest="$repo_root/.git/hooks/pre-commit"

chmod +x "$repo_root/scripts/secret_scan.sh" "$hook_src"

mkdir -p "$repo_root/.git/hooks"
if [[ -e "$hook_dest" && ! -L "$hook_dest" ]]; then
  cp "$hook_dest" "$hook_dest.bak" 2>/dev/null || true
  echo "[setup-hooks] Existing pre-commit backed up to .git/hooks/pre-commit.bak"
fi

ln -sf "$hook_src" "$hook_dest"
echo "[setup-hooks] Pre-commit hook installed -> .git/hooks/pre-commit"
echo "[setup-hooks] Ensure gitleaks is installed: https://github.com/gitleaks/gitleaks#installation"

