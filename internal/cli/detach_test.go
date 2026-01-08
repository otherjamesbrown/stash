package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestDetachCommand tests the detach command
func TestDetachCommand(t *testing.T) {
	t.Run("detach file from record", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record that has an attachment
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

		// When: User runs `stash detach <id> document.txt`
		rootCmd.SetArgs([]string{"detach", recordID, "document.txt"})
		err := rootCmd.Execute()

		// Then: File is removed from files/<record-id>/
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify file was removed
		attachedPath := filepath.Join(tempDir, ".stash", "inventory", "files", recordID, "document.txt")
		if _, err := os.Stat(attachedPath); err == nil {
			t.Error("expected attached file to be removed")
		}
	})

	t.Run("detach file with --json output", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record that has an attachment
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

		// When: User runs `stash detach <id> document.txt --json`
		rootCmd.SetArgs([]string{"detach", recordID, "document.txt", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if jsonOutput["record_id"] != recordID {
			t.Errorf("expected record_id '%s', got %v", recordID, jsonOutput["record_id"])
		}
		if jsonOutput["filename"] != "document.txt" {
			t.Errorf("expected filename 'document.txt', got %v", jsonOutput["filename"])
		}
		if jsonOutput["detached"] != true {
			t.Errorf("expected detached to be true, got %v", jsonOutput["detached"])
		}
	})

	t.Run("reject non-existent attachment", func(t *testing.T) {
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

		// When: User runs `stash detach <id> nonexistent.txt`
		rootCmd.SetArgs([]string{"detach", recordID, "nonexistent.txt"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for non-existent attachment, got %d", ExitCode)
		}
	})

	t.Run("reject non-existent record", func(t *testing.T) {
		// Given: Stash "inventory" exists but no record
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash detach inv-fake document.txt`
		rootCmd.SetArgs([]string{"detach", "inv-fake", "document.txt"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for non-existent record, got %d", ExitCode)
		}
	})
}

// TestDetachCommand_MustNot tests anti-requirements for detach command
func TestDetachCommand_MustNot(t *testing.T) {
	t.Run("must not detach from deleted record", func(t *testing.T) {
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

		// When: User tries to detach file from deleted record
		rootCmd.SetArgs([]string{"detach", recordID, "document.txt"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for deleted record, got %d", ExitCode)
		}
	})
}
