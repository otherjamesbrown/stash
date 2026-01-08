package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestEnv sets up the test environment and returns a cleanup function
func setupTestEnv(t *testing.T) (tempDir string, cleanup func()) {
	t.Helper()
	tempDir = t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)

	// Mock the exit function to capture exit code instead of exiting
	origExitFunc := ExitFunc
	ExitFunc = func(code int) {
		ExitCode = code
		// Don't actually exit in tests
	}
	ExitCode = 0 // Reset exit code

	cleanup = func() {
		os.Chdir(origDir)
		ExitFunc = origExitFunc
		ExitCode = 0
	}
	return tempDir, cleanup
}

// TestUC_ST_001_InitStash tests UC-ST-001: Initialize Stash
func TestUC_ST_001_InitStash(t *testing.T) {
	t.Run("AC-01: create stash with required fields", func(t *testing.T) {
		// Given: No stash named "inventory" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash init inventory --prefix inv-`
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		err := rootCmd.Execute()

		// Then: Directory .stash/inventory/ is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		stashDir := filepath.Join(tempDir, ".stash", "inventory")
		if _, err := os.Stat(stashDir); os.IsNotExist(err) {
			t.Error("expected .stash/inventory/ directory to exist")
		}

		// Then: config.json contains name, prefix, created_at, created_by
		configPath := filepath.Join(stashDir, "config.json")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config.json: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			t.Fatalf("failed to parse config.json: %v", err)
		}

		if config["name"] != "inventory" {
			t.Errorf("expected name 'inventory', got %v", config["name"])
		}
		if config["prefix"] != "inv-" {
			t.Errorf("expected prefix 'inv-', got %v", config["prefix"])
		}
		if config["created"] == nil {
			t.Error("expected created field to be set")
		}
		if config["created_by"] == nil {
			t.Error("expected created_by field to be set")
		}

		// Then: Empty records.jsonl is created
		recordsPath := filepath.Join(stashDir, "records.jsonl")
		if _, err := os.Stat(recordsPath); os.IsNotExist(err) {
			t.Error("expected records.jsonl to exist")
		}

		// Then: files/ subdirectory is created
		filesDir := filepath.Join(stashDir, "files")
		if _, err := os.Stat(filesDir); os.IsNotExist(err) {
			t.Error("expected files/ subdirectory to exist")
		}
	})

	t.Run("AC-02: reject duplicate stash name", func(t *testing.T) {
		// Given: Stash "inventory" already exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash first
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Read original config
		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		origConfig, _ := os.ReadFile(configPath)

		// Reset exit code
		ExitCode = 0

		// When: User runs `stash init inventory --prefix inv-` again
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}

		// Then: No files are modified
		newConfig, _ := os.ReadFile(configPath)
		if string(origConfig) != string(newConfig) {
			t.Error("expected config.json to remain unchanged")
		}
	})

	t.Run("AC-03: reject invalid prefix - too short", func(t *testing.T) {
		// Given: No stash named "test" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash init test --prefix x` (too short)
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "x"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}

		// Then: No files are created
		stashDir := filepath.Join(tempDir, ".stash", "test")
		if _, err := os.Stat(stashDir); err == nil {
			t.Error("expected .stash/test/ directory NOT to exist for invalid prefix")
		}
	})

	t.Run("AC-03: reject invalid prefix - missing dash", func(t *testing.T) {
		// Given: No stash named "test" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash init test --prefix abc` (missing dash)
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "abc"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}

		// Then: No files should be created
		stashDir := filepath.Join(tempDir, ".stash", "test")
		if _, err := os.Stat(stashDir); err == nil {
			t.Error("expected .stash/test/ directory NOT to exist for invalid prefix")
		}
	})

	t.Run("AC-04: skip daemon with flag", func(t *testing.T) {
		// Given: No stash named "test" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash init test --prefix ts- --no-daemon`
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "ts-", "--no-daemon"})
		err := rootCmd.Execute()

		// Then: Stash is created successfully
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		stashDir := filepath.Join(tempDir, ".stash", "test")
		if _, err := os.Stat(stashDir); os.IsNotExist(err) {
			t.Error("expected .stash/test/ directory to exist")
		}
	})

	t.Run("AC-05: capture actor and branch", func(t *testing.T) {
		// Given: User has STASH_ACTOR set
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		os.Setenv("STASH_ACTOR", "testuser")
		defer os.Unsetenv("STASH_ACTOR")

		// When: User runs `stash init inventory --prefix inv-`
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: config.json created_by is "testuser"
		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config.json: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			t.Fatalf("failed to parse config.json: %v", err)
		}

		if config["created_by"] != "testuser" {
			t.Errorf("expected created_by 'testuser', got %v", config["created_by"])
		}
	})
}

// TestUC_ST_001_InitStash_MustNot tests anti-requirements
func TestUC_ST_001_InitStash_MustNot(t *testing.T) {
	t.Run("must not overwrite existing stash", func(t *testing.T) {
		// Given: Stash "inventory" exists with config
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash first
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Read original config
		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		origConfig, _ := os.ReadFile(configPath)

		// Reset exit code
		ExitCode = 0

		// When: User tries to create again with different prefix
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "abc-"})
		rootCmd.Execute() // This should fail with exit code 1

		// Then: Exit code should be 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}

		// Then: Original config should be unchanged
		newConfig, _ := os.ReadFile(configPath)
		if string(origConfig) != string(newConfig) {
			t.Error("expected config.json to remain unchanged when stash already exists")
		}
	})

	t.Run("must not create stash with invalid prefix", func(t *testing.T) {
		// Given: No stash exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs with invalid prefix
		invalidPrefixes := []string{"x", "ab", "ABCD-", "abc1-", "abcde-f"}
		for _, prefix := range invalidPrefixes {
			ExitCode = 0 // Reset for each iteration
			rootCmd.SetArgs([]string{"init", "test", "--prefix", prefix})
			rootCmd.Execute()

			// Then: Exit code should be 2
			if ExitCode != 2 {
				t.Errorf("expected exit code 2 for invalid prefix %q, got %d", prefix, ExitCode)
			}

			// Then: No stash should be created
			stashDir := filepath.Join(tempDir, ".stash", "test")
			if _, err := os.Stat(stashDir); err == nil {
				t.Errorf("expected .stash/test/ NOT to exist for invalid prefix %q", prefix)
				os.RemoveAll(stashDir) // Clean up for next iteration
			}
		}
	})
}
