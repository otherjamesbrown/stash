package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_003_ShowRecord tests UC-REC-003: Show Record
func TestUC_REC_003_ShowRecord(t *testing.T) {
	t.Run("AC-01: show record details", func(t *testing.T) {
		// Given: Record inv-ex4j exists with fields
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record with fields
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash show inv-ex4j`
		rootCmd.SetArgs([]string{"show", recordID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output shows record ID and hash
		if !strings.Contains(output, recordID) {
			t.Error("expected output to contain record ID")
		}
		if !strings.Contains(output, "Hash") {
			t.Error("expected output to contain hash")
		}

		// Then: Output shows created/updated timestamps and actors
		if !strings.Contains(output, "Created") {
			t.Error("expected output to contain Created")
		}
		if !strings.Contains(output, "Updated") {
			t.Error("expected output to contain Updated")
		}

		// Then: Output shows all user fields
		if !strings.Contains(output, "Name") {
			t.Error("expected output to contain Name field")
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected output to contain Laptop value")
		}
		if !strings.Contains(output, "Price") {
			t.Error("expected output to contain Price field")
		}

		// Then: Output lists children section
		if !strings.Contains(output, "Children") {
			t.Error("expected output to contain Children section")
		}
	})

	t.Run("AC-02: JSON output format", func(t *testing.T) {
		// Given: Record inv-ex4j exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()

		// Create a child record
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", recordID})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash show inv-ex4j --json`
		rootCmd.SetArgs([]string{"show", recordID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON object
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		// Then: Contains all system and user fields
		if jsonOutput["_id"] == nil {
			t.Error("expected _id in JSON output")
		}
		if jsonOutput["_hash"] == nil {
			t.Error("expected _hash in JSON output")
		}
		if jsonOutput["_created_at"] == nil {
			t.Error("expected _created_at in JSON output")
		}
		if jsonOutput["_created_by"] == nil {
			t.Error("expected _created_by in JSON output")
		}
		if jsonOutput["Name"] == nil {
			t.Error("expected Name in JSON output")
		}
		if jsonOutput["Price"] == nil {
			t.Error("expected Price in JSON output")
		}

		// Then: Contains _children array
		if jsonOutput["_children"] == nil {
			t.Error("expected _children in JSON output")
		}
		children, ok := jsonOutput["_children"].([]interface{})
		if !ok {
			t.Error("expected _children to be an array")
		}
		if len(children) != 1 {
			t.Errorf("expected 1 child, got %d", len(children))
		}
	})

	t.Run("AC-03: show with file contents", func(t *testing.T) {
		// Given: Record inv-ex4j has attached file
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID and create a file
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Create attached file
		filesDir := filepath.Join(tempDir, ".stash", "inventory", "files")
		os.MkdirAll(filesDir, 0755)
		filePath := filepath.Join(filesDir, recordID+".md")
		os.WriteFile(filePath, []byte("# Test Content\n\nThis is test content."), 0644)

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash show inv-ex4j --with-files`
		rootCmd.SetArgs([]string{"show", recordID, "--with-files"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output includes inline file contents
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "Files") {
			t.Error("expected output to contain Files section")
		}
		if !strings.Contains(output, "Test Content") {
			t.Error("expected output to contain file contents")
		}
	})

	t.Run("AC-04: show with history", func(t *testing.T) {
		// Given: Record inv-ex4j has been updated multiple times
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		// Update the record
		ExitCode = 0
		rootCmd.SetArgs([]string{"set", recordID, "Price=999"})
		rootCmd.Execute()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash show inv-ex4j --history`
		rootCmd.SetArgs([]string{"show", recordID, "--history"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output includes change history
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Shows timestamp, operation, actor, branch for each change
		if !strings.Contains(output, "History") {
			t.Error("expected output to contain History section")
		}
		if !strings.Contains(output, "Timestamp") {
			t.Error("expected output to contain Timestamp header")
		}
		if !strings.Contains(output, "Operation") {
			t.Error("expected output to contain Operation header")
		}
		if !strings.Contains(output, "Actor") {
			t.Error("expected output to contain Actor header")
		}
	})

	t.Run("reject non-existent record", func(t *testing.T) {
		// Given: No record inv-fake exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash show inv-fake`
		rootCmd.SetArgs([]string{"show", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("reject deleted record", func(t *testing.T) {
		// Given: Record is soft-deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create and delete record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0

		// When: User runs `stash show inv-ex4j`
		rootCmd.SetArgs([]string{"show", recordID})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})
}

// TestUC_REC_003_ShowRecord_MustNot tests anti-requirements
func TestUC_REC_003_ShowRecord_MustNot(t *testing.T) {
	t.Run("must not modify any data", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		origHash := records[0].Hash
		origUpdatedAt := records[0].UpdatedAt
		store.Close()

		ExitCode = 0

		// When: User runs show command
		rootCmd.SetArgs([]string{"show", recordID})
		rootCmd.Execute()

		// Then: Record should not be modified
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)
		if rec.Hash != origHash {
			t.Error("show command should not modify hash")
		}
		if !rec.UpdatedAt.Equal(origUpdatedAt) {
			t.Error("show command should not modify updated_at")
		}
	})
}
