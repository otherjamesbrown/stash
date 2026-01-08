package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestHierarchy_ParentChildCreation tests parent-child record creation workflows
func TestHierarchy_ParentChildCreation(t *testing.T) {
	t.Run("create multiple children with sequential IDs", func(t *testing.T) {
		// Given: Parent record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// When: Create multiple children
		for _, name := range []string{"Charger", "Case", "Mouse"} {
			resetFlags()
			rootCmd.SetArgs([]string{"add", name, "--parent", parentID})
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("failed to add child %s: %v", name, err)
			}
		}

		// Then: Children have sequential IDs
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		children, _ := store.GetChildren("inventory", parentID)
		if len(children) != 3 {
			t.Fatalf("expected 3 children, got %d", len(children))
		}

		expectedIDs := []string{parentID + ".1", parentID + ".2", parentID + ".3"}
		for i, child := range children {
			if child.ID != expectedIDs[i] {
				t.Errorf("expected child ID %s, got %s", expectedIDs[i], child.ID)
			}
		}
	})

	t.Run("create grandchildren", func(t *testing.T) {
		// Given: Parent and child exist
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

		childID := parentID + ".1"

		// When: Create grandchild
		resetFlags()
		rootCmd.SetArgs([]string{"add", "USB Cable", "--parent", childID})
		err := rootCmd.Execute()

		// Then: Grandchild has correct ID
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		grandchildren, _ := store.GetChildren("inventory", childID)
		if len(grandchildren) != 1 {
			t.Fatalf("expected 1 grandchild, got %d", len(grandchildren))
		}

		expectedGrandchildID := childID + ".1"
		if grandchildren[0].ID != expectedGrandchildID {
			t.Errorf("expected grandchild ID %s, got %s", expectedGrandchildID, grandchildren[0].ID)
		}
	})
}

// TestHierarchy_ListWithParent tests listing records with --parent filter
func TestHierarchy_ListWithParent(t *testing.T) {
	t.Run("list direct children only with --parent", func(t *testing.T) {
		// Given: Parent with children and grandchildren
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create children
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		childID := parentID + ".1"

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Case", "--parent", parentID})
		rootCmd.Execute()

		// Create grandchild
		resetFlags()
		rootCmd.SetArgs([]string{"add", "USB Cable", "--parent", childID})
		rootCmd.Execute()

		// When: List with --parent
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		children, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: parentID})

		// Then: Only direct children are listed (not grandchildren)
		if len(children) != 2 {
			t.Errorf("expected 2 direct children, got %d", len(children))
		}

		for _, child := range children {
			if !strings.HasPrefix(child.ID, parentID+".") {
				t.Errorf("unexpected child ID: %s", child.ID)
			}
			// Should not contain grandchildren
			if strings.Count(child.ID, ".") > 1 {
				t.Errorf("grandchild found in direct children list: %s", child.ID)
			}
		}
	})

	t.Run("list all records including nested with --all", func(t *testing.T) {
		// Given: Hierarchy exists
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

		// Create grandchild
		resetFlags()
		childID := parentID + ".1"
		rootCmd.SetArgs([]string{"add", "USB Cable", "--parent", childID})
		rootCmd.Execute()

		// When: List with --all
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		allRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})

		// Then: All records including nested are listed
		if len(allRecords) != 3 {
			t.Errorf("expected 3 total records, got %d", len(allRecords))
		}
	})
}

// TestHierarchy_ShowWithChildren tests showing a record displays children
func TestHierarchy_ShowWithChildren(t *testing.T) {
	t.Run("show includes children list", func(t *testing.T) {
		// Given: Parent with children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create children
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Case", "--parent", parentID})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: Show parent with --json
		rootCmd.SetArgs([]string{"show", parentID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output includes _children array
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		children, ok := jsonOutput["_children"].([]interface{})
		if !ok {
			t.Fatal("expected _children array in output")
		}

		if len(children) != 2 {
			t.Errorf("expected 2 children in output, got %d", len(children))
		}
	})
}

// TestHierarchy_CascadeDelete tests cascade deletion of parent with children
func TestHierarchy_CascadeDelete(t *testing.T) {
	t.Run("cascade deletes nested hierarchy", func(t *testing.T) {
		// Given: Parent with children and grandchildren
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

		childID := parentID + ".1"

		// Create grandchild
		resetFlags()
		rootCmd.SetArgs([]string{"add", "USB Cable", "--parent", childID})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: Delete parent with --cascade
		rootCmd.SetArgs([]string{"rm", parentID, "--cascade", "--yes"})
		err := rootCmd.Execute()

		// Then: All records are deleted
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// All should be deleted
		allRecords, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(allRecords) != 0 {
			t.Errorf("expected 0 active records, got %d", len(allRecords))
		}

		// Verify all are marked deleted
		allIncludingDeleted, _ := store.ListRecords("inventory", storage.ListOptions{
			ParentID:       "*",
			IncludeDeleted: true,
		})
		if len(allIncludingDeleted) != 3 {
			t.Errorf("expected 3 total records, got %d", len(allIncludingDeleted))
		}
		for _, rec := range allIncludingDeleted {
			if !rec.IsDeleted() {
				t.Errorf("expected record %s to be deleted", rec.ID)
			}
		}
	})
}
