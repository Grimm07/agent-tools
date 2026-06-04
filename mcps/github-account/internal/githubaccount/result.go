package githubaccount

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult wraps a value as a pretty-printed JSON text content block, the
// human-readable half of a tool result.
func textResult(v any) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

// mapGitHubError converts a go-github error into a clear, non-panic error that
// distinguishes not-found / forbidden / rate-limit from generic failures, so the
// agent can tell "not found" from "rate limited". No stack traces leak.
func mapGitHubError(err error) error {
	var rl *github.RateLimitError
	if errors.As(err, &rl) {
		return fmt.Errorf("github rate limit exceeded; resets at %s", rl.Rate.Reset.Time)
	}
	var er *github.ErrorResponse
	if errors.As(err, &er) {
		return fmt.Errorf("github %d: %s", er.Response.StatusCode, er.Message)
	}
	return err
}
