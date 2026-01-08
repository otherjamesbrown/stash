package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupStash creates a temporary directory with initialized stash.
// Returns path to temp dir (cleaned up automatically via t.Cleanup).
func SetupStash(t *testing.T, name, prefix string) string {
	t.Helper()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "stash-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Initialize stash in temp directory (use --no-daemon for tests)
	MustSucceedInDir(t, tmpDir, "init", name, "--prefix", prefix, "--no-daemon")

	return tmpDir
}

// SetupStashWithColumns creates stash with predefined columns.
// Each column string is just the column name; a default description will be used.
func SetupStashWithColumns(t *testing.T, name, prefix string, columns []string) string {
	t.Helper()

	tmpDir := SetupStash(t, name, prefix)

	// Add each column
	for _, col := range columns {
		MustSucceedInDir(t, tmpDir, "column", "add", col)
	}

	return tmpDir
}

// SetupStashWithRecords creates stash with columns and sample records.
// The columns parameter specifies column names.
// The records parameter is a slice of maps where keys are column names and values are field values.
func SetupStashWithRecords(t *testing.T, name, prefix string, columns []string, records []map[string]string) string {
	t.Helper()

	tmpDir := SetupStashWithColumns(t, name, prefix, columns)

	// Add each record
	for _, rec := range records {
		args := []string{"add"}

		// First column value is the primary value (positional argument)
		if len(columns) > 0 {
			if val, ok := rec[columns[0]]; ok {
				args = append(args, val)
			}
		}

		// Additional columns use --set flags
		for i := 1; i < len(columns); i++ {
			col := columns[i]
			if val, ok := rec[col]; ok {
				args = append(args, "--set", col+"="+val)
			}
		}

		MustSucceedInDir(t, tmpDir, args...)
	}

	return tmpDir
}

// TempDir creates a temporary directory that is cleaned up after the test.
// This is useful for tests that need a clean directory without initializing a stash.
func TempDir(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "stash-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return tmpDir
}

// StashDir returns the .stash directory path for a given stash.
func StashDir(baseDir, stashName string) string {
	return filepath.Join(baseDir, ".stash", stashName)
}

// ConfigPath returns the path to the config.json file for a given stash.
func ConfigPath(baseDir, stashName string) string {
	return filepath.Join(StashDir(baseDir, stashName), "config.json")
}

// RecordsPath returns the path to the records.jsonl file for a given stash.
func RecordsPath(baseDir, stashName string) string {
	return filepath.Join(StashDir(baseDir, stashName), "records.jsonl")
}
