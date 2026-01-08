package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_QRY_002_ListChildren tests UC-QRY-002: List Children
func TestUC_QRY_002_ListChildren(t *testing.T) {
	t.Run("AC-01: list direct children", func(t *testing.T) {
		// Given: Record inv-a1b2 has children inv-a1b2.1 and inv-a1b2.2
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

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash children inv-a1b2`
		rootCmd.SetArgs([]string{"children", parentID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Both children are listed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should contain both children
		if !strings.Contains(output, parentID+".1") {
			t.Error("expected output to contain first child ID")
		}
		if !strings.Contains(output, parentID+".2") {
			t.Error("expected output to contain second child ID")
		}

		// Then: Grandchildren are NOT listed (direct children only)
		if strings.Contains(output, parentID+".1.1") {
			t.Error("grandchild should not be listed")
		}
	})

	t.Run("AC-02: JSON output", func(t *testing.T) {
		// Given: Record inv-a1b2 has children
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

		// When: User runs `stash children inv-a1b2 --json`
		rootCmd.SetArgs([]string{"children", parentID, "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON array
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var jsonOutput []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		// Then: Each child includes _id and user fields
		if len(jsonOutput) != 2 {
			t.Errorf("expected 2 children in JSON output, got %d", len(jsonOutput))
		}

		for _, child := range jsonOutput {
			if child["_id"] == nil {
				t.Error("expected _id in child")
			}
			if child["Name"] == nil {
				t.Error("expected Name field in child")
			}
		}
	})

	t.Run("AC-03: empty result for no children", func(t *testing.T) {
		// Given: Record inv-ex4j has no children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create record without children
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

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

		// When: User runs `stash children inv-ex4j`
		rootCmd.SetArgs([]string{"children", recordID})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Empty result shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should indicate no children
		if !strings.Contains(output, "No children") && !strings.Contains(output, "0") {
			// JSON output would be empty array
			if strings.TrimSpace(output) != "[]" && !strings.Contains(strings.ToLower(output), "no children") {
				t.Errorf("expected empty result message, got: %s", output)
			}
		}
	})
}

// TestUC_QRY_002_ListChildren_MustNot tests anti-requirements
func TestUC_QRY_002_ListChildren_MustNot(t *testing.T) {
	t.Run("must not include grandchildren", func(t *testing.T) {
		// Given: Parent with child and grandchild
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

		grandchildID := childID + ".1"

		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: List children of parent
		rootCmd.SetArgs([]string{"children", parentID, "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Grandchild should not be included
		var jsonOutput []map[string]interface{}
		json.Unmarshal([]byte(output), &jsonOutput)

		if len(jsonOutput) != 1 {
			t.Errorf("expected 1 direct child, got %d", len(jsonOutput))
		}

		for _, child := range jsonOutput {
			if child["_id"] == grandchildID {
				t.Error("grandchild should not be in direct children list")
			}
		}
	})
}

// TestChildren_Errors tests error cases for children command
func TestChildren_Errors(t *testing.T) {
	t.Run("reject non-existent parent", func(t *testing.T) {
		// Given: No record exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: List children of non-existent record
		rootCmd.SetArgs([]string{"children", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})
}
