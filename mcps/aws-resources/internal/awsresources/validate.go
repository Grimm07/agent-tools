package awsresources

import (
	"errors"
	"fmt"
)

// errClusterRequired is returned by tools that need an ECS cluster argument.
var errClusterRequired = errors.New("cluster is required for this tool (name or ARN)")

// regionRequired returns an error if region is empty. Regional tools call this
// before any AWS request so a missing region fails fast with a clear message
// rather than surfacing as an opaque SDK endpoint error.
func regionRequired(region string) error {
	if region == "" {
		return fmt.Errorf("region is required for this tool (e.g. us-east-1)")
	}
	return nil
}

// clampMax normalizes a caller-supplied max_items value: non-positive means use
// the default (100), and anything above the hard cap (1000) is clamped.
func clampMax(n int) int {
	if n <= 0 {
		return 100
	}
	if n > 1000 {
		return 1000
	}
	return n
}
