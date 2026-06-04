#!/usr/bin/env bash
# PostToolUse hook: format files by extension after Edit/Write.
# Reads the tool-call JSON on stdin; formats only the touched file.
# Keeps formatting tool-driven (gofmt/ruff/tofu) per CLAUDE.md. Never blocks.
set -euo pipefail

file="$(python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null || true)"
[ -n "$file" ] || exit 0
[ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file" >/dev/null 2>&1 || true
    ;;
  *.py)
    command -v ruff >/dev/null 2>&1 && ruff format -q "$file" >/dev/null 2>&1 || true
    ;;
  *.tf)
    command -v tofu >/dev/null 2>&1 && tofu fmt "$file" >/dev/null 2>&1 || true
    ;;
esac
exit 0
