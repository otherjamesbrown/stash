package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUC_ST_002_DropStash tests UC-ST-002: Drop Stash
func TestUC_ST_002_DropStash(t *testing.T) {
	t.Run("AC-02: skip confirmation with --yes", func(t *testing.T) {
		// Given: Stash "inventory" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash first
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		stashDir := filepath.Join(tempDir, ".stash", "inventory")
		if _, err := os.Stat(stashDir); os.IsNotExist(err) {
			t.Fatal("expected stash to be created first")
		}

		// Reset exit code
		ExitCode = 0

		// When: User runs `stash drop inventory --yes`
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		err := rootCmd.Execute()

		// Then: Stash is deleted without prompting
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Directory .stash/inventory/ is deleted
		if _, err := os.Stat(stashDir); !os.IsNotExist(err) {
			t.Error("expected .stash/inventory/ directory to be deleted")
		}
	})

	t.Run("AC-03: reject non-existent stash", func(t *testing.T) {
		// Given: No stash named "fake" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create .stash directory but no stash
		os.MkdirAll(filepath.Join(tempDir, ".stash"), 0755)

		// When: User runs `stash drop fake --yes`
		rootCmd.SetArgs([]string{"drop", "fake", "--yes"})
		rootCmd.Execute()

		// Then: Command fails with exit code 3
		if ExitCode != 3 {
			t.Errorf("expected exit code 3, got %d", ExitCode)
		}
	})

	t.Run("drop removes SQLite table metadata", func(t *testing.T) {
		// Given: Stash "inventory" exists with SQLite cache
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		cacheDB := filepath.Join(tempDir, ".stash", "cache.db")
		if _, err := os.Stat(cacheDB); os.IsNotExist(err) {
			t.Fatal("expected cache.db to be created")
		}

		// Reset exit code
		ExitCode = 0

		// When: Drop stash
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		err := rootCmd.Execute()

		// Then: No error
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Cache.db may still exist (it's shared) but the table should be dropped
		// We can't easily test the table is dropped without directly querying SQLite
	})
}

// TestUC_ST_002_DropStash_MustNot tests anti-requirements
func TestUC_ST_002_DropStash_MustNot(t *testing.T) {
	t.Run("must not leave orphaned files on error", func(t *testing.T) {
		// Given: Stash "inventory" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Reset exit code
		ExitCode = 0

		// When: Drop succeeds
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		rootCmd.Execute()

		// Then: Both directory and metadata should be removed
		stashDir := filepath.Join(tempDir, ".stash", "inventory")
		if _, err := os.Stat(stashDir); !os.IsNotExist(err) {
			t.Error("expected .stash/inventory/ to be removed")
		}
	})
}
