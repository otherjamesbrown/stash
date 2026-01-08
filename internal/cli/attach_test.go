package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestAttachCommand tests the attach command
func TestAttachCommand(t *testing.T) {
	t.Run("attach file to record (copy mode)", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create record: %v", err)
		}

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Create a test file to attach
		testFile := filepath.Join(tempDir, "document.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		ExitCode = 0
		resetFlags()

		// When: User runs `stash attach <id> document.txt`
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		err := rootCmd.Execute()

		// Then: File is copied to files/<record-id>/document.txt
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify file was copied
		attachedPath := filepath.Join(tempDir, ".stash", "inventory", "files", recordID, "document.txt")
		if _, err := os.Stat(attachedPath); os.IsNotExist(err) {
			t.Error("expected attached file to exist")
		}

		// Verify original file still exists (copy mode)
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("expected original file to still exist in copy mode")
		}
	})

	t.Run("attach file with --move flag", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create record: %v", err)
		}

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Create a test file to attach
		testFile := filepath.Join(tempDir, "document.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		ExitCode = 0
		resetFlags()

		// When: User runs `stash attach <id> document.txt --move`
		rootCmd.SetArgs([]string{"attach", recordID, testFile, "--move"})
		err := rootCmd.Execute()

		// Then: File is moved to files/<record-id>/document.txt
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify file was moved
		attachedPath := filepath.Join(tempDir, ".stash", "inventory", "files", recordID, "document.txt")
		if _, err := os.Stat(attachedPath); os.IsNotExist(err) {
			t.Error("expected attached file to exist")
		}

		// Verify original file no longer exists (move mode)
		if _, err := os.Stat(testFile); err == nil {
			t.Error("expected original file to be removed in move mode")
		}
	})

	t.Run("attach file with --json output", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record
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

		// Create a test file
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash attach <id> document.txt --json`
		rootCmd.SetArgs([]string{"attach", recordID, testFile, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON with attachment metadata
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if jsonOutput["record_id"] != recordID {
			t.Errorf("expected record_id '%s', got %v", recordID, jsonOutput["record_id"])
		}
		if jsonOutput["name"] != "document.txt" {
			t.Errorf("expected name 'document.txt', got %v", jsonOutput["name"])
		}
		if jsonOutput["hash"] == nil {
			t.Error("expected hash in JSON output")
		}
		if jsonOutput["size"] == nil {
			t.Error("expected size in JSON output")
		}
	})

	t.Run("reject non-existent file", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record
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

		// When: User runs `stash attach <id> nonexistent.txt`
		rootCmd.SetArgs([]string{"attach", recordID, filepath.Join(tempDir, "nonexistent.txt")})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for non-existent file, got %d", ExitCode)
		}
	})

	t.Run("reject non-existent record", func(t *testing.T) {
		// Given: Stash "inventory" exists but no record
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a test file
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		// When: User runs `stash attach inv-fake document.txt`
		rootCmd.SetArgs([]string{"attach", "inv-fake", testFile})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for non-existent record, got %d", ExitCode)
		}
	})

	t.Run("reject duplicate attachment", func(t *testing.T) {
		// Given: Stash "inventory" exists with a record that already has an attachment
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

		// Create and attach first file
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		rootCmd.Execute()

		// Create another file with same name
		os.WriteFile(testFile, []byte("different content"), 0644)

		ExitCode = 0
		resetFlags()

		// When: User tries to attach another file with same name
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1 for duplicate attachment, got %d", ExitCode)
		}
	})
}

// TestAttachCommand_MustNot tests anti-requirements for attach command
func TestAttachCommand_MustNot(t *testing.T) {
	t.Run("must not attach to deleted record", func(t *testing.T) {
		// Given: Record exists but is deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create and delete a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		// Create a test file
		testFile := filepath.Join(tempDir, "document.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		ExitCode = 0
		resetFlags()

		// When: User tries to attach file to deleted record
		rootCmd.SetArgs([]string{"attach", recordID, testFile})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4 for deleted record, got %d", ExitCode)
		}
	})
}
