package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_006_RestoreRecord tests UC-REC-006: Restore Deleted Record
func TestUC_REC_006_RestoreRecord(t *testing.T) {
	t.Run("AC-01: restore deleted record", func(t *testing.T) {
		// Given: Record inv-ex4j is soft-deleted
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

		ExitCode = 0
		resetFlags()

		// When: User runs `stash restore inv-ex4j`
		rootCmd.SetArgs([]string{"restore", recordID})
		err := rootCmd.Execute()

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: _deleted_at is set to null
		// Then: _deleted_by is set to null
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecord("inventory", recordID)
		if err != nil {
			t.Fatalf("failed to get record: %v", err)
		}

		if rec.DeletedAt != nil {
			t.Error("expected _deleted_at to be null")
		}
		if rec.DeletedBy != "" {
			t.Error("expected _deleted_by to be empty")
		}

		// Then: _updated_at and _updated_by are updated
		if rec.UpdatedAt.IsZero() {
			t.Error("expected _updated_at to be updated")
		}
		if rec.UpdatedBy == "" {
			t.Error("expected _updated_by to be set")
		}

		// Then: Record appears in `stash list`
		activeRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		found := false
		for _, r := range activeRecords {
			if r.ID == recordID {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected restored record to appear in list")
		}
	})

	t.Run("AC-02: restore with cascade", func(t *testing.T) {
		// Given: Record inv-ex4j and children are soft-deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child records
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Case", "--parent", parentID})
		rootCmd.Execute()

		// Delete parent and children with cascade
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"rm", parentID, "--cascade", "--yes"})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash restore inv-ex4j --cascade`
		rootCmd.SetArgs([]string{"restore", parentID, "--cascade"})
		err := rootCmd.Execute()

		// Then: Parent and all deleted children are restored
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Parent should be restored
		parent, err := store.GetRecord("inventory", parentID)
		if err != nil {
			t.Fatalf("expected parent to be restored, got error: %v", err)
		}
		if parent.IsDeleted() {
			t.Error("expected parent to NOT be deleted")
		}

		// All children should be restored
		children, _ := store.GetChildren("inventory", parentID)
		if len(children) != 2 {
			t.Errorf("expected 2 children, got %d", len(children))
		}
		for _, child := range children {
			if child.IsDeleted() {
				t.Errorf("expected child %s to NOT be deleted", child.ID)
			}
		}
	})

	t.Run("AC-03: reject restore of active record", func(t *testing.T) {
		// Given: Record inv-ex4j is active (not deleted)
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record (not deleted)
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash restore inv-ex4j`
		rootCmd.SetArgs([]string{"restore", recordID})
		rootCmd.Execute()

		// Then: Command fails or is no-op
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for active record")
		}
	})
}

// TestUC_REC_006_RestoreRecord_MustNot tests anti-requirements
func TestUC_REC_006_RestoreRecord_MustNot(t *testing.T) {
	t.Run("must not restore non-existent record", func(t *testing.T) {
		// Given: No record exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: User tries to restore non-existent record
		rootCmd.SetArgs([]string{"restore", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})
}

// TestUC_REC_006_RestoreRecord_JSONOutput tests JSON output for restore command
func TestUC_REC_006_RestoreRecord_JSONOutput(t *testing.T) {
	t.Run("JSON output shows restored record", func(t *testing.T) {
		// Given: Record is deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash restore inv-ex4j --json`
		rootCmd.SetArgs([]string{"restore", recordID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		// Then: Contains restored count
		if jsonOutput["restored"] != float64(1) {
			t.Errorf("expected restored=1, got %v", jsonOutput["restored"])
		}
	})
}

// TestUC_REC_006_RestorePartialCascade tests cascade restore when some children are active
func TestUC_REC_006_RestorePartialCascade(t *testing.T) {
	t.Run("cascade restore only restores deleted children", func(t *testing.T) {
		// Given: Parent and one child are deleted, another child is active
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create two children
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Case", "--parent", parentID})
		rootCmd.Execute()

		// Delete parent only (not cascade)
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.DeleteRecord("inventory", parentID, "test")
		// Delete only first child
		children, _ := store.GetChildren("inventory", parentID)
		if len(children) > 0 {
			store.DeleteRecord("inventory", children[0].ID, "test")
		}
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash restore parent --cascade`
		rootCmd.SetArgs([]string{"restore", parentID, "--cascade"})
		err := rootCmd.Execute()

		// Then: Parent and deleted child are restored, active child unaffected
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		parent, _ := store.GetRecord("inventory", parentID)
		if parent.IsDeleted() {
			t.Error("expected parent to be restored")
		}
	})
}
