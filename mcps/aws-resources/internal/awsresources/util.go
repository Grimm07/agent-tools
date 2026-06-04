package awsresources

import (
	"strconv"
	"time"
)

// formatTime renders a *time.Time as RFC3339 in UTC, or "" if nil. Used for the
// trimmed projections so output is stable and JSON-friendly.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// itoa is a thin alias for strconv.Itoa, kept local so projection code reads
// cleanly when composing endpoint strings.
func itoa(n int) string { return strconv.Itoa(n) }
