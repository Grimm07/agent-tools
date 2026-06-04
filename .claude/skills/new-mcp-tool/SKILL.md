---
name: new-mcp-tool
description: Add a new read-only tool to an existing Go MCP unit in mcps/, following the repo's established handler pattern (TDD, free-function handler, trimmed projection, server registration).
disable-model-invocation: true
argument-hint: "<unit> <tool_name> — e.g. mcps/aws-resources list_kms_keys"
allowed-tools: Read, Grep, Glob, Edit, Write, Bash
---

Add one new tool to an existing Go MCP server under `mcps/<unit>/`, matching the
patterns already in that unit. This is the same shape every tool in `github-account`
and `aws-resources` uses — replicate it; do not invent a new structure.

## Inputs
- **unit**: the MCP unit dir (e.g. `mcps/aws-resources`).
- **tool_name**: snake_case MCP tool name (e.g. `list_kms_keys`).
If either is missing, ask once, then proceed.

## Steps (TDD — test first, always)

1. **Study the unit.** Read 1–2 existing handler files and their tests in
   `mcps/<unit>/internal/<pkg>/` to copy the exact idioms: how inputs/outputs are
   structured (`json` + `jsonschema` tags), how the client/config is obtained, how
   errors are mapped (`mapGitHubError` / `mapAWSError`), how results are returned
   (`textResult`), and the test fake style (httptest `BaseURL` for go-github; SDK
   `*APIClient`/narrow interface fakes for aws-sdk-go-v2).

2. **Write the failing test first** in the matching `*_test.go`, using the unit's
   established fake style. Assert on the trimmed projection. Run it; confirm it FAILS
   (`go test -race ./...`).

3. **Implement the handler** as a free function taking the client/config (not closing
   over server state) so it unit-tests directly. Define `XxxInput`/`XxxOutput` structs
   with a **trimmed projection** — only the fields an agent needs, never the raw
   SDK/library struct. Map errors through the unit's existing error helper.

4. **Run the test; confirm it PASSES.**

5. **Register the tool** in `server.go` via the same thin-closure `mcp.AddTool(...)`
   pattern as the others.

6. **Enforce read-only** (these units are read-only): use only `Get/List/Describe`
   calls. Verify with
   `grep -rE '(Create|Delete|Put|Update|Modify|Terminate)[A-Z]' internal/`
   (filter the benign `CreateDate` field).

7. **Verify & document.** Run `make test` and `make lint` (green). Add the tool to the
   unit's README tool table.

8. **Commit** with `feat(<unit-slug>): <tool_name>`.

## Guardrails
- Match the unit's existing naming, file layout, and projection style exactly — consistency over cleverness.
- Offline tests only: never call a live API in a unit test.
- If a new third-party dependency seems needed, stop and confirm with the user first (the repo prizes a small, justified dependency surface).
