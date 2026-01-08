// Package context provides context detection for the stash CLI.
// It handles actor resolution, git branch detection, and stash location finding.
package context

import "os"

// ResolveActor returns the actor name following priority order:
// 1. flagValue (--actor flag) if non-empty
// 2. $STASH_ACTOR environment variable if set
// 3. $USER environment variable if set
// 4. "unknown" as fallback
func ResolveActor(flagValue string) string {
	// Priority 1: Flag value
	if flagValue != "" {
		return flagValue
	}

	// Priority 2: STASH_ACTOR environment variable
	if actor := os.Getenv("STASH_ACTOR"); actor != "" {
		return actor
	}

	// Priority 3: USER environment variable
	if user := os.Getenv("USER"); user != "" {
		return user
	}

	// Priority 4: Fallback
	return "unknown"
}
