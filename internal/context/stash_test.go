package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindStashDir(t *testing.T) {
	t.Run("finds .stash in current directory", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Change to the temp directory
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		result := FindStashDir()
		assert.Equal(t, stashDir, result)
	})

	t.Run("finds .stash in parent directory", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		subDir := filepath.Join(tmpDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		// Change to the subdirectory
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(subDir)

		result := FindStashDir()
		assert.Equal(t, stashDir, result)
	})

	t.Run("finds .stash in deeply nested parent", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		deepDir := filepath.Join(tmpDir, "a", "b", "c", "d")
		require.NoError(t, os.MkdirAll(deepDir, 0755))

		// Change to the deep directory
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(deepDir)

		result := FindStashDir()
		assert.Equal(t, stashDir, result)
	})

	t.Run("returns empty when no .stash found", func(t *testing.T) {
		// Create temp directory without .stash
		tmpDir := t.TempDir()

		// Change to the temp directory
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		result := FindStashDir()
		assert.Empty(t, result)
	})
}

func TestFindStashDirFrom(t *testing.T) {
	t.Run("finds .stash from specified directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		result := findStashDirFrom(tmpDir)
		assert.Equal(t, stashDir, result)
	})

	t.Run("returns empty for non-existent directory", func(t *testing.T) {
		result := findStashDirFrom("/nonexistent/path/12345")
		assert.Empty(t, result)
	})
}

func TestDefaultStash(t *testing.T) {
	// Save original environment
	origStashDefault := os.Getenv("STASH_DEFAULT")
	defer os.Setenv("STASH_DEFAULT", origStashDefault)

	t.Run("priority 1: STASH_DEFAULT env var", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create multiple stashes
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		os.Setenv("STASH_DEFAULT", "my-default-stash")

		result := DefaultStash(stashDir)
		assert.Equal(t, "my-default-stash", result)
	})

	t.Run("priority 2: single stash auto-detected", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create exactly one stash
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "only-stash"), 0755))

		result := DefaultStash(stashDir)
		assert.Equal(t, "only-stash", result)
	})

	t.Run("priority 3: empty when multiple stashes exist", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create multiple stashes
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		result := DefaultStash(stashDir)
		assert.Empty(t, result)
	})

	t.Run("returns empty when stashDir is empty", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		result := DefaultStash("")
		assert.Empty(t, result)
	})

	t.Run("returns empty when no stashes exist", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		// No stash subdirectories created

		result := DefaultStash(stashDir)
		assert.Empty(t, result)
	})

	t.Run("ignores hidden directories", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create one visible stash and one hidden directory
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "visible-stash"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, ".hidden"), 0755))

		result := DefaultStash(stashDir)
		assert.Equal(t, "visible-stash", result)
	})

	t.Run("ignores files (only counts directories)", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create one stash and one file
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "my-stash"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(stashDir, "config.json"), []byte("{}"), 0644))

		result := DefaultStash(stashDir)
		assert.Equal(t, "my-stash", result)
	})
}

func TestListStashes(t *testing.T) {
	t.Run("returns empty slice for nonexistent directory", func(t *testing.T) {
		result := listStashes("/nonexistent/path")
		assert.Nil(t, result)
	})

	t.Run("returns stash names", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "alpha"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "beta"), 0755))

		result := listStashes(stashDir)
		assert.Len(t, result, 2)
		assert.Contains(t, result, "alpha")
		assert.Contains(t, result, "beta")
	})
}

func TestIsHiddenFile(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{".hidden", true},
		{".git", true},
		{"visible", false},
		{"file.txt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHiddenFile(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindStashDir_EdgeCases(t *testing.T) {
	t.Run("returns empty when .stash is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .stash as a file, not a directory
		stashFile := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.WriteFile(stashFile, []byte("not a directory"), 0644))

		// Change to the temp directory
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		result := FindStashDir()
		assert.Empty(t, result)
	})

	t.Run("finds .stash in grandparent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		deepDir := filepath.Join(tmpDir, "level1", "level2", "level3")
		require.NoError(t, os.MkdirAll(deepDir, 0755))

		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(deepDir)

		result := FindStashDir()
		assert.Equal(t, stashDir, result)
	})
}

func TestDefaultStash_EdgeCases(t *testing.T) {
	origStashDefault := os.Getenv("STASH_DEFAULT")
	defer os.Setenv("STASH_DEFAULT", origStashDefault)

	t.Run("STASH_DEFAULT takes precedence even if stash does not exist", func(t *testing.T) {
		os.Setenv("STASH_DEFAULT", "nonexistent-stash")

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "actual-stash"), 0755))

		result := DefaultStash(stashDir)
		// STASH_DEFAULT wins even if the stash doesn't exist
		assert.Equal(t, "nonexistent-stash", result)
	})

	t.Run("returns empty when stashDir is invalid path", func(t *testing.T) {
		os.Unsetenv("STASH_DEFAULT")

		result := DefaultStash("/this/path/does/not/exist/at/all")
		assert.Empty(t, result)
	})
}

func TestListStashes_EdgeCases(t *testing.T) {
	t.Run("ignores hidden directories starting with dot", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create normal stash
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "normal"), 0755))

		// Create hidden directory
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, ".hidden"), 0755))

		os.Unsetenv("STASH_DEFAULT")

		result := DefaultStash(stashDir)
		// Should auto-select "normal" since ".hidden" is ignored
		assert.Equal(t, "normal", result)
	})

	t.Run("does not auto-select when multiple visible stashes", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.Mkdir(stashDir, 0755))

		// Create multiple visible stashes
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash1"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, "stash2"), 0755))

		// Create hidden directory (should be ignored)
		require.NoError(t, os.Mkdir(filepath.Join(stashDir, ".hidden"), 0755))

		os.Unsetenv("STASH_DEFAULT")

		result := DefaultStash(stashDir)
		// Should be empty since there are multiple visible stashes
		assert.Empty(t, result)
	})
}
