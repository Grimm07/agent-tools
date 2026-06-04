package awsresources

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult renders v as indented JSON wrapped in a single text content block.
// It pairs with the typed output struct each handler also returns, giving
// clients both a human-readable and a structured view.
func textResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		// Marshalling our own trimmed structs cannot realistically fail; fall
		// back to a stringified form rather than panicking.
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%+v", v)}}}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

// mapAWSError turns an AWS SDK error into a readable, actionable tool error
// carrying the service name and (when available) the API error code. Common
// failure modes — access denied and throttling — get extra guidance. The
// original error is wrapped so callers retain the full chain.
func mapAWSError(service string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		msg := apiErr.ErrorMessage()
		switch {
		case strings.Contains(code, "AccessDenied") || strings.Contains(code, "UnauthorizedOperation") || code == "AuthFailure":
			return fmt.Errorf("%s: access denied (%s): %s — check the profile's IAM read permissions", service, code, msg)
		case strings.Contains(code, "Throttling") || code == "RequestLimitExceeded" || code == "TooManyRequestsException":
			return fmt.Errorf("%s: throttled (%s): %s — retry shortly", service, code, msg)
		default:
			return fmt.Errorf("%s: %s: %s", service, code, msg)
		}
	}
	return fmt.Errorf("%s: %w", service, err)
}
