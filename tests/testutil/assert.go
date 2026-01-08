package testutil

import (
	"fmt"
	"strings"
	"testing"
)

// AssertRecordExists verifies a record with given ID exists using the CLI.
func AssertRecordExists(t *testing.T, dir, stashName, id string) {
	t.Helper()

	// Use CLI to show the record - it will fail if record doesn't exist or is deleted
	result := RunStashInDir(t, dir, "show", id, "--json")
	if result.ExitCode != 0 {
		t.Fatalf("expected record with ID %s to exist, but show failed with exit code %d", id, result.ExitCode)
	}

	// Parse JSON to verify it's not deleted
	record := ParseJSONObject(t, result.Stdout)
	if GetFieldBool(record, "_deleted") {
		t.Fatalf("expected record with ID %s to exist, but it is soft-deleted", id)
	}
}

// AssertRecordDeleted verifies a record is soft-deleted using the CLI.
func AssertRecordDeleted(t *testing.T, dir, stashName, id string) {
	t.Helper()

	// Use CLI list with --deleted to find deleted records
	result := RunStashInDir(t, dir, "list", "--deleted", "--json", "--stash", stashName)
	if result.ExitCode != 0 {
		t.Fatalf("failed to list deleted records: exit code %d", result.ExitCode)
	}

	records := ParseJSONOutput(t, result.Stdout)
	found := false
	for _, rec := range records {
		if GetField(rec, "_id") == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected record with ID %s to be soft-deleted, but it was not found in deleted list", id)
	}
}

// AssertRecordNotExists verifies a record with given ID does not exist at all using the CLI.
func AssertRecordNotExists(t *testing.T, dir, stashName, id string) {
	t.Helper()

	// Use CLI to show the record - it should fail with exit code 4 (not found)
	result := RunStashInDir(t, dir, "show", id, "--json")
	if result.ExitCode == 0 {
		t.Fatalf("expected record with ID %s to not exist, but show succeeded", id)
	}
	// Exit code 4 means record not found, which is what we expect
}

// AssertColumnExists verifies a column exists in config.
func AssertColumnExists(t *testing.T, dir, stashName, columnName string) {
	t.Helper()

	config := ReadConfig(t, dir, stashName)
	columnNames := GetColumnNames(config)

	for _, name := range columnNames {
		if strings.EqualFold(name, columnName) {
			return
		}
	}

	t.Fatalf("expected column %s to exist, but it was not found. Existing columns: %v", columnName, columnNames)
}

// AssertColumnNotExists verifies a column does not exist in config.
func AssertColumnNotExists(t *testing.T, dir, stashName, columnName string) {
	t.Helper()

	config := ReadConfig(t, dir, stashName)
	columnNames := GetColumnNames(config)

	for _, name := range columnNames {
		if strings.EqualFold(name, columnName) {
			t.Fatalf("expected column %s to not exist, but it was found", columnName)
		}
	}
}

// AssertExitCode checks the exit code of a result.
func AssertExitCode(t *testing.T, result Result, expected int) {
	t.Helper()

	if result.ExitCode != expected {
		t.Fatalf("expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			expected, result.ExitCode, result.Stdout, result.Stderr)
	}
}

// AssertContains checks stdout contains substring.
func AssertContains(t *testing.T, result Result, substr string) {
	t.Helper()

	if !strings.Contains(result.Stdout, substr) {
		t.Fatalf("expected stdout to contain %q, but it didn't\nstdout: %s", substr, result.Stdout)
	}
}

// AssertStderrContains checks stderr contains substring.
func AssertStderrContains(t *testing.T, result Result, substr string) {
	t.Helper()

	if !strings.Contains(result.Stderr, substr) {
		t.Fatalf("expected stderr to contain %q, but it didn't\nstderr: %s", substr, result.Stderr)
	}
}

// AssertNotContains checks stdout does not contain substring.
func AssertNotContains(t *testing.T, result Result, substr string) {
	t.Helper()

	if strings.Contains(result.Stdout, substr) {
		t.Fatalf("expected stdout to not contain %q, but it did\nstdout: %s", substr, result.Stdout)
	}
}

// AssertEmpty checks stdout is empty.
func AssertEmpty(t *testing.T, result Result) {
	t.Helper()

	if result.Stdout != "" {
		t.Fatalf("expected stdout to be empty, but got: %s", result.Stdout)
	}
}

// AssertNotEmpty checks stdout is not empty.
func AssertNotEmpty(t *testing.T, result Result) {
	t.Helper()

	if result.Stdout == "" {
		t.Fatal("expected stdout to not be empty")
	}
}

// AssertRecordCount checks the number of active records in the stash using CLI.
func AssertRecordCount(t *testing.T, dir, stashName string, expected int) {
	t.Helper()

	result := RunStashInDir(t, dir, "list", "--json", "--stash", stashName)
	if result.ExitCode != 0 {
		t.Fatalf("failed to list records: exit code %d", result.ExitCode)
	}

	records := ParseJSONOutput(t, result.Stdout)
	if len(records) != expected {
		t.Fatalf("expected %d active records, got %d", expected, len(records))
	}
}

// AssertRecordField checks a specific field value on a record using CLI.
// Compares values as strings, handling both string and numeric JSON values.
func AssertRecordField(t *testing.T, dir, stashName, id, field, expected string) {
	t.Helper()

	result := RunStashInDir(t, dir, "show", id, "--json")
	if result.ExitCode != 0 {
		t.Fatalf("record with ID %s not found (exit code %d)", id, result.ExitCode)
	}

	record := ParseJSONObject(t, result.Stdout)

	// Get the field value as string (handling both string and numeric types)
	var actual string
	if val, ok := record[field]; ok {
		switch v := val.(type) {
		case string:
			actual = v
		case float64:
			// Convert float to string, handling integers properly
			if v == float64(int64(v)) {
				actual = fmt.Sprintf("%d", int64(v))
			} else {
				actual = fmt.Sprintf("%g", v)
			}
		default:
			actual = fmt.Sprintf("%v", v)
		}
	}

	if actual != expected {
		t.Fatalf("expected field %s on record %s to be %q, got %q", field, id, expected, actual)
	}
}

// AssertFileExists checks that a file exists at the given path.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if !FileExists(t, path) {
		t.Fatalf("expected file %s to exist, but it doesn't", path)
	}
}

// AssertFileNotExists checks that a file does not exist at the given path.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if FileExists(t, path) {
		t.Fatalf("expected file %s to not exist, but it does", path)
	}
}

// AssertDirExists checks that a directory exists at the given path.
func AssertDirExists(t *testing.T, path string) {
	t.Helper()

	if !DirExists(t, path) {
		t.Fatalf("expected directory %s to exist, but it doesn't", path)
	}
}

// AssertDirNotExists checks that a directory does not exist at the given path.
func AssertDirNotExists(t *testing.T, path string) {
	t.Helper()

	if DirExists(t, path) {
		t.Fatalf("expected directory %s to not exist, but it does", path)
	}
}

// AssertStashInitialized checks that a stash has been initialized in a directory.
func AssertStashInitialized(t *testing.T, dir, stashName string) {
	t.Helper()

	stashDir := StashDir(dir, stashName)
	AssertDirExists(t, stashDir)
	AssertFileExists(t, ConfigPath(dir, stashName))
}
