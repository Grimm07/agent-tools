#!/usr/bin/env bash
# PreToolUse hook: block Edit/Write to state/secret files before it happens.
# Defense beyond .gitignore, per CLAUDE.md ("never commit state or secrets").
# Exit 2 blocks the tool call and shows the stderr reason to the model.
set -euo pipefail

file="$(python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null || true)"
[ -n "$file" ] || exit 0

case "$file" in
  *.tfvars.example) exit 0 ;;  # committed examples are allowed
esac

case "$file" in
  *.tfstate|*.tfstate.*|*.tfvars|*.pem|*.env|*.env.*)
    echo "Blocked: '$(basename "$file")' is a state/secret file — never edit it via the agent (see CLAUDE.md). Edit it manually if you truly intend to." >&2
    exit 2
    ;;
esac
exit 0
