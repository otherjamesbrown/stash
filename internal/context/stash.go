package context

import (
	"os"
	"path/filepath"
)

const stashDirName = ".stash"

// FindStashDir returns the path to .stash directory
// Searches current directory and parents up to root or git repo boundary
// Returns empty string if not found
func FindStashDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return findStashDirFrom(dir)
}

// findStashDirFrom searches for .stash starting from the given directory
// and walking up to the root or git repo boundary.
func findStashDirFrom(startDir string) string {
	dir := startDir
	for {
		stashPath := filepath.Join(dir, stashDirName)
		if info, err := os.Stat(stashPath); err == nil && info.IsDir() {
			return stashPath
		}

		// Check for git boundary - stop searching if we hit a .git directory
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			// We found a .git but no .stash in or above it
			// Continue searching in case .stash is above .git
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			return ""
		}
		dir = parent
	}
}

// DefaultStash returns the default stash name:
// 1. $STASH_DEFAULT environment variable if set
// 2. Only stash if exactly one exists
// 3. Empty string (requires --stash flag)
func DefaultStash(stashDir string) string {
	// Priority 1: STASH_DEFAULT environment variable
	if defaultStash := os.Getenv("STASH_DEFAULT"); defaultStash != "" {
		return defaultStash
	}

	// Priority 2: Only stash if exactly one exists
	if stashDir == "" {
		return ""
	}

	stashes := listStashes(stashDir)
	if len(stashes) == 1 {
		return stashes[0]
	}

	// Priority 3: Empty string (requires --stash flag)
	return ""
}

// listStashes returns a list of stash names in the given stash directory.
// Each stash is a subdirectory within .stash/
func listStashes(stashDir string) []string {
	entries, err := os.ReadDir(stashDir)
	if err != nil {
		return nil
	}

	var stashes []string
	for _, entry := range entries {
		if entry.IsDir() && !isHiddenFile(entry.Name()) {
			stashes = append(stashes, entry.Name())
		}
	}
	return stashes
}

// isHiddenFile returns true if the filename starts with a dot
func isHiddenFile(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
