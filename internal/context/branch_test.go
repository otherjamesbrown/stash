package context

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectBranch(t *testing.T) {
	t.Run("returns branch name in git repository", func(t *testing.T) {
		// We're running in the stash repo, so this should return a branch name
		branch := DetectBranch()
		// Should be a non-empty string (could be "main", "master", or feature branch)
		assert.NotEmpty(t, branch, "should detect a branch in git repository")
	})

	t.Run("returns empty string outside git repository", func(t *testing.T) {
		// Change to a directory that's not a git repo
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)

		// /tmp is typically not a git repository
		err := os.Chdir("/tmp")
		if err != nil {
			t.Skip("cannot change to /tmp")
		}

		branch := DetectBranch()
		assert.Empty(t, branch, "should return empty string outside git repo")
	})
}

func TestIsGitRepo(t *testing.T) {
	t.Run("returns true in git repository", func(t *testing.T) {
		// We're running in the stash repo
		result := IsGitRepo()
		assert.True(t, result, "should detect we're in a git repository")
	})

	t.Run("returns false outside git repository", func(t *testing.T) {
		// Change to a directory that's not a git repo
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)

		// /tmp is typically not a git repository
		err := os.Chdir("/tmp")
		if err != nil {
			t.Skip("cannot change to /tmp")
		}

		result := IsGitRepo()
		assert.False(t, result, "should return false outside git repo")
	})
}
