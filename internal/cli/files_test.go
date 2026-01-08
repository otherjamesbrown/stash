package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestFilesCommand tests the files command
func TestFilesCommand(t *testing.T) {
	t.Run("list files for record with attachments", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record that has attachments
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Create and attach test files
		testFile1 := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile1, []byte("test content 1"), 0644)
		testFile2 := filepath.Join(tempDir, "image.png")
		os.WriteFile(testFile2, []byte("fake image data"), 0644)

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"attach", recordID, testFile1})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"attach", recordID, testFile2})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash files <id>`
		rootCmd.SetArgs([]string{"files", recordID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output lists both files
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		if !strings.Contains(output, "document.txt") {
			t.Error("expected output to contain 'document.txt'")
		}
		if !strings.Contains(output, "image.png") {
			t.Error("expected output to contain 'image.png'")
		}
	})

	t.Run("list files for record with no attachments", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record but no attachments
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash files <id>`
		rootCmd.SetArgs([]string{"files", recordID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output indicates no files
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		if !strings.Contains(output, "No files") {
			t.Errorf("expected output to indicate no files, got: %s", output)
		}
	})

	t.Run("list files with --json output", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record that has attachments
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Create and attach a test file
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash files <id> --json`
		rootCmd.SetArgs([]string{"files", recordID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON with file list
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if jsonOutput["record_id"] != recordID {
			t.Errorf("expected record_id '%s', got %v", recordID, jsonOutput["record_id"])
		}
		if jsonOutput["count"].(float64) != 1 {
			t.Errorf("expected count 1, got %v", jsonOutput["count"])
		}

		files, ok := jsonOutput["files"].([]interface{})
		if !ok || len(files) != 1 {
			t.Errorf("expected files array with 1 element, got %v", jsonOutput["files"])
		}

		file := files[0].(map[string]interface{})
		if file["name"] != "document.txt" {
			t.Errorf("expected name 'document.txt', got %v", file["name"])
		}
	})

	t.Run("reject non-existent record", func(t *testing.T) {
		// Given: Stash "inventory" exists but no record
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash files inv-fake`
		rootCmd.SetArgs([]string{"files", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for non-existent record, got %d", ExitCode)
		}
	})
}

// TestFilesCommand_MustNot tests anti-requirements for files command
func TestFilesCommand_MustNot(t *testing.T) {
	t.Run("must not list files for deleted record", func(t *testing.T) {
		// Given: Record exists but is deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID

		// Attach a file before deleting
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		rootCmd.Execute()

		// Delete the record
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User tries to list files for deleted record
		rootCmd.SetArgs([]string{"files", recordID})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for deleted record, got %d", ExitCode)
		}
	})
}

// TestFormatSize tests the formatSize helper function
func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		result := formatSize(tc.bytes)
		if result != tc.expected {
			t.Errorf("formatSize(%d) = %s, expected %s", tc.bytes, result, tc.expected)
		}
	}
}
