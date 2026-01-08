package context

import (
	"os/exec"
	"strings"
)

// DetectBranch returns the current git branch, or empty string if:
// - Not in a git repository
// - git command not available
// - Any error occurs (fail gracefully)
func DetectBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "" // Not in a git repo, or git not available
	}
	return strings.TrimSpace(string(out))
}

// IsGitRepo returns true if current directory is in a git repository
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
