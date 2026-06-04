# GitHub Account

A go-mcp unit.

An [MCP](https://modelcontextprotocol.io) server built on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).

**Language: Go.** This unit is an MCP server meant to ship as a single static binary and talk to
clients over stdio — exactly the case the repo reserves for Go (see the root `CLAUDE.md`).

## Requirements

- Go ≥ 1.25 (the SDK requires it). With `GOTOOLCHAIN=auto`, the right toolchain downloads on first build.

## Commands

```bash
make test    # go test -race ./...
make build   # -> bin/github-account
make run     # serve over stdio
make lint    # gofmt check + go vet + staticcheck (if installed)
make tidy    # go mod tidy
```

## Layout

```
cmd/github-account/main.go            # entrypoint: wires stdio transport
internal/githubaccount/server.go        # NewServer() + tool handlers
internal/githubaccount/server_test.go   # handler unit test + in-memory transport test
```

## Adding a tool

1. Define an input struct with `json` + `jsonschema` field tags, and (optionally) an output struct.
2. Write a handler `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)` — keep it a
   plain function so it stays directly unit-testable.
3. Register it in `NewServer()` with `mcp.AddTool(s, &mcp.Tool{Name: ..., Description: ...}, Handler)`.
4. Test it two ways: call the handler directly, and round-trip it through
   `mcp.NewInMemoryTransports()` (see `server_test.go`).

## Updating from the template

```bash
copier update --trust   # pulls template improvements; answers are in .copier-answers.yml
```
