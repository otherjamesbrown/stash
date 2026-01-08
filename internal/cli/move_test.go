package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestMove_BasicMove tests basic move functionality
func TestMove_BasicMove(t *testing.T) {
	t.Run("move record to new parent", func(t *testing.T) {
		// Given: Two parent records and a child of one
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create first parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parent1ID := records[0].ID
		store.Close()

		// Create second parent
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Desktop"})
		rootCmd.Execute()

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ = store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		var parent2ID string
		for _, r := range records {
			if r.ID != parent1ID {
				parent2ID = r.ID
				break
			}
		}
		store.Close()

		// Create child of first parent
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parent1ID})
		rootCmd.Execute()

		oldChildID := parent1ID + ".1"

		ExitCode = 0
		resetFlags()

		// When: Move child to second parent
		rootCmd.SetArgs([]string{"move", oldChildID, "--parent", parent2ID})
		err := rootCmd.Execute()

		// Then: Child has new ID under new parent
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Old ID should not exist
		_, err = store.GetRecord("inventory", oldChildID)
		if err == nil {
			t.Error("expected old ID to not exist")
		}

		// New ID should exist under new parent
		newChildID := parent2ID + ".1"
		newChild, err := store.GetRecord("inventory", newChildID)
		if err != nil {
			t.Fatalf("expected new ID to exist, got error: %v", err)
		}
		if newChild.ParentID != parent2ID {
			t.Errorf("expected parent %s, got %s", parent2ID, newChild.ParentID)
		}
		if newChild.Fields["Name"] != "Charger" {
			t.Errorf("expected Name=Charger, got %v", newChild.Fields["Name"])
		}
	})

	t.Run("move record to root (no parent)", func(t *testing.T) {
		// Given: Parent with child
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		oldChildID := parentID + ".1"

		ExitCode = 0
		resetFlags()

		// Capture stdout to get new ID
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: Move child to root (empty parent)
		rootCmd.SetArgs([]string{"move", oldChildID, "--parent", ""})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Child becomes root record with new ID
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Should have 2 root records now
		rootRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: ""})
		if len(rootRecords) != 2 {
			t.Errorf("expected 2 root records, got %d", len(rootRecords))
		}

		// Output should contain new ID
		if output == "" {
			t.Error("expected output with new ID")
		}
	})

	t.Run("move record with children updates all IDs", func(t *testing.T) {
		// Given: Parent with child and grandchild
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create first parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parent1ID := records[0].ID
		store.Close()

		// Create second parent
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Desktop"})
		rootCmd.Execute()

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ = store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		var parent2ID string
		for _, r := range records {
			if r.ID != parent1ID {
				parent2ID = r.ID
				break
			}
		}
		store.Close()

		// Create child
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parent1ID})
		rootCmd.Execute()

		childID := parent1ID + ".1"

		// Create grandchild
		resetFlags()
		rootCmd.SetArgs([]string{"add", "USB Cable", "--parent", childID})
		rootCmd.Execute()

		grandchildID := childID + ".1"

		ExitCode = 0
		resetFlags()

		// When: Move child (with grandchild) to second parent
		rootCmd.SetArgs([]string{"move", childID, "--parent", parent2ID})
		err := rootCmd.Execute()

		// Then: Both child and grandchild have new IDs
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Old IDs should not exist
		_, err = store.GetRecord("inventory", childID)
		if err == nil {
			t.Error("expected old child ID to not exist")
		}

		_, err = store.GetRecord("inventory", grandchildID)
		if err == nil {
			t.Error("expected old grandchild ID to not exist")
		}

		// New IDs should exist
		newChildID := parent2ID + ".1"
		newGrandchildID := newChildID + ".1"

		newChild, err := store.GetRecord("inventory", newChildID)
		if err != nil {
			t.Fatalf("expected new child ID to exist: %v", err)
		}
		if newChild.Fields["Name"] != "Charger" {
			t.Errorf("expected child Name=Charger, got %v", newChild.Fields["Name"])
		}

		newGrandchild, err := store.GetRecord("inventory", newGrandchildID)
		if err != nil {
			t.Fatalf("expected new grandchild ID to exist: %v", err)
		}
		if newGrandchild.Fields["Name"] != "USB Cable" {
			t.Errorf("expected grandchild Name='USB Cable', got %v", newGrandchild.Fields["Name"])
		}
	})
}

// TestMove_Errors tests move error cases
func TestMove_Errors(t *testing.T) {
	t.Run("reject move to non-existent parent", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: Move to non-existent parent
		rootCmd.SetArgs([]string{"move", recordID, "--parent", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("reject move non-existent record", func(t *testing.T) {
		// Given: No record exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: Move non-existent record
		rootCmd.SetArgs([]string{"move", "inv-fake", "--parent", ""})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("reject move to self as parent", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: Move record to itself
		rootCmd.SetArgs([]string{"move", recordID, "--parent", recordID})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when moving to self")
		}
	})

	t.Run("reject move to own descendant", func(t *testing.T) {
		// Given: Parent with child
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		childID := parentID + ".1"

		ExitCode = 0
		resetFlags()

		// When: Move parent to be child of its own child
		rootCmd.SetArgs([]string{"move", parentID, "--parent", childID})
		rootCmd.Execute()

		// Then: Command fails (would create cycle)
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when creating cycle")
		}
	})
}

// TestMove_JSONOutput tests JSON output for move command
func TestMove_JSONOutput(t *testing.T) {
	t.Run("JSON output shows old and new IDs", func(t *testing.T) {
		// Given: Parent with child
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create two parents
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parent1ID := records[0].ID
		store.Close()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Desktop"})
		rootCmd.Execute()

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ = store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		var parent2ID string
		for _, r := range records {
			if r.ID != parent1ID {
				parent2ID = r.ID
				break
			}
		}
		store.Close()

		// Create child
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parent1ID})
		rootCmd.Execute()

		oldChildID := parent1ID + ".1"

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: Move with --json
		rootCmd.SetArgs([]string{"move", oldChildID, "--parent", parent2ID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: JSON output includes old_id and new_id
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if jsonOutput["old_id"] != oldChildID {
			t.Errorf("expected old_id=%s, got %v", oldChildID, jsonOutput["old_id"])
		}

		expectedNewID := parent2ID + ".1"
		if jsonOutput["new_id"] != expectedNewID {
			t.Errorf("expected new_id=%s, got %v", expectedNewID, jsonOutput["new_id"])
		}
	})
}
