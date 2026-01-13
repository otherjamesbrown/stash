package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// resetFlags resets global command flags for test isolation
func resetFlags() {
	// Reset add command flags
	addSetFlags = nil
	addParentID = ""
	// Reset set command flags
	setColFlags = nil
	setAutoCreate = false
	// Reset show command flags
	showWithFiles = false
	showHistory = false
	// Reset list command flags
	listAll = false
	listDeleted = false
	listParent = ""
	listLimit = 0
	listOffset = 0
	listOrderBy = ""
	listDesc = false
	listWhere = nil
	listSearch = ""
	listColumns = ""
	// Reset count command flags
	countAll = false
	countDeleted = false
	countWhere = nil
	// Reset rm command flags
	rmCascade = false
	rmYes = false
	// Reset restore command flags
	restoreCascade = false
	// Reset purge command flags
	purgeID = ""
	purgeBefore = ""
	purgeAll = false
	purgeDryRun = false
	purgeYes = false
	// Reset history command flags
	historyBy = ""
	historySince = ""
	historyLimit = 0
	// Reset attach command flags
	attachMove = false
	// Reset move command flags
	moveParentID = ""
	// Reset init-claude command flags
	forceInstall = false
	// Reset query command flags
	queryCSV = false
	queryNoHeaders = false
	queryColumns = ""
	// Reset bulk-set command flags
	bulkSetWhere = nil
	bulkSetSet = nil
	// Reset search command flags
	searchIn = nil
	// Reset status command flags
	statusProcessing = true
	statusAgent = ""
	// Reset global flags
	jsonOutput = false
	stashName = ""
	actorName = ""
	quiet = false
	verbose = false
	noDaemon = false
}

// setupTestStashWithColumns creates a test stash with columns for testing
func setupTestStashWithColumns(t *testing.T, stashName, prefix string, columns []string) (tempDir string, cleanup func()) {
	t.Helper()
	tempDir, baseCleanup := setupTestEnv(t)
	resetFlags()

	// Create stash
	rootCmd.SetArgs([]string{"init", stashName, "--prefix", prefix})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to init stash: %v", err)
	}

	// Add columns directly via storage
	store, err := storage.NewStore(filepath.Join(tempDir, ".stash"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	for _, colName := range columns {
		col := model.Column{
			Name:    colName,
			Added:   time.Now(),
			AddedBy: "test",
		}
		if err := store.AddColumn(stashName, col); err != nil {
			t.Fatalf("failed to add column %s: %v", colName, err)
		}
	}

	ExitCode = 0 // Reset exit code
	resetFlags()

	cleanup = func() {
		resetFlags()
		baseCleanup()
	}
	return tempDir, cleanup
}

// TestUC_REC_001_AddRecord tests UC-REC-001: Add Record
func TestUC_REC_001_AddRecord(t *testing.T) {
	t.Run("AC-01: add record with primary value", func(t *testing.T) {
		// Given: Stash "inventory" exists with column "Name"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash add "Laptop"`
		rootCmd.SetArgs([]string{"add", "Laptop"})
		err := rootCmd.Execute()

		// Then: Record is created with unique ID (inv-xxxx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify record exists in storage
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, err := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if err != nil {
			t.Fatalf("failed to list records: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		rec := records[0]

		// Then: ID starts with inv-
		if !strings.HasPrefix(rec.ID, "inv-") {
			t.Errorf("expected ID to start with 'inv-', got %s", rec.ID)
		}

		// Then: Name field is set to "Laptop"
		if rec.Fields["Name"] != "Laptop" {
			t.Errorf("expected Name='Laptop', got %v", rec.Fields["Name"])
		}

		// Then: _hash is calculated from user fields
		if rec.Hash == "" {
			t.Error("expected _hash to be set")
		}

		// Then: _created_at and _updated_at are set
		if rec.CreatedAt.IsZero() {
			t.Error("expected _created_at to be set")
		}
		if rec.UpdatedAt.IsZero() {
			t.Error("expected _updated_at to be set")
		}

		// Then: _created_by and _updated_by are set
		if rec.CreatedBy == "" {
			t.Error("expected _created_by to be set")
		}
		if rec.UpdatedBy == "" {
			t.Error("expected _updated_by to be set")
		}
	})

	t.Run("AC-02: add record with additional fields", func(t *testing.T) {
		// Given: Stash "inventory" exists with columns "Name", "Price", "Category"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})
		defer cleanup()

		// When: User runs `stash add "Laptop" --set Price=999 --set Category="electronics"`
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999", "--set", "Category=electronics"})
		err := rootCmd.Execute()

		// Then: Record is created with all three fields set
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify record has all fields
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		rec := records[0]
		if fmt.Sprintf("%v", rec.Fields["Name"]) != "Laptop" {
			t.Errorf("expected Name='Laptop', got %v", rec.Fields["Name"])
		}
		if fmt.Sprintf("%v", rec.Fields["Price"]) != "999" {
			t.Errorf("expected Price='999', got %v", rec.Fields["Price"])
		}
		if fmt.Sprintf("%v", rec.Fields["Category"]) != "electronics" {
			t.Errorf("expected Category='electronics', got %v", rec.Fields["Category"])
		}
	})

	t.Run("AC-03: add child record", func(t *testing.T) {
		// Given: Record inv-ex4j exists in "inventory"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent record
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Laptop"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create parent: %v", err)
		}

		// Get parent ID
		store, err := storage.NewStore(filepath.Join(tempDir, ".stash"))
		if err != nil {
			t.Fatalf("failed to open store: %v", err)
		}
		records, err := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if err != nil {
			t.Fatalf("failed to list records: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("expected parent record to exist")
		}
		parentID := records[0].ID
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash add "Charger" --parent inv-ex4j`
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		err = rootCmd.Execute()

		// Then: Child record is created with ID inv-ex4j.1
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify child record
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		children, _ := store.GetChildren("inventory", parentID)
		if len(children) != 1 {
			t.Fatalf("expected 1 child, got %d", len(children))
		}

		child := children[0]
		expectedChildID := parentID + ".1"
		if child.ID != expectedChildID {
			t.Errorf("expected child ID '%s', got '%s'", expectedChildID, child.ID)
		}
		if child.ParentID != parentID {
			t.Errorf("expected _parent='%s', got '%s'", parentID, child.ParentID)
		}
	})

	t.Run("AC-04: reject invalid parent", func(t *testing.T) {
		// Given: No record inv-fake exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash add "Charger" --parent inv-fake`
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", "inv-fake"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("AC-05: JSON output format", func(t *testing.T) {
		// Given: Stash "inventory" exists with column "Name"
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash add "Laptop" --json`
		rootCmd.SetArgs([]string{"add", "Laptop", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON object
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		// Then: Contains _id, _hash, _created_by, _branch, Name
		if jsonOutput["_id"] == nil {
			t.Error("expected _id in JSON output")
		}
		if jsonOutput["_hash"] == nil {
			t.Error("expected _hash in JSON output")
		}
		if jsonOutput["_created_by"] == nil {
			t.Error("expected _created_by in JSON output")
		}
		if jsonOutput["Name"] == nil {
			t.Error("expected Name in JSON output")
		}
	})

	t.Run("AC-06: reject empty primary value", func(t *testing.T) {
		// Given: Stash "inventory" exists with column "Name"
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash add ""`
		rootCmd.SetArgs([]string{"add", ""})
		rootCmd.Execute()

		// Then: Command fails with appropriate error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for empty value")
		}
	})

	t.Run("AC-07: trim whitespace from values", func(t *testing.T) {
		// Given: Stash "inventory" exists with column "Name"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash add "  Laptop  "`
		rootCmd.SetArgs([]string{"add", "  Laptop  "})
		err := rootCmd.Execute()

		// Then: Record is created with Name = "Laptop" (trimmed)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify trimmed value
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0].Fields["Name"] != "Laptop" {
			t.Errorf("expected Name='Laptop' (trimmed), got '%v'", records[0].Fields["Name"])
		}
	})
}

// TestUC_REC_001_AddRecord_MustNot tests anti-requirements
func TestUC_REC_001_AddRecord_MustNot(t *testing.T) {
	t.Run("must not create record without columns", func(t *testing.T) {
		// Given: Stash "inventory" exists with no columns
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash without columns
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()
		ExitCode = 0

		// When: User tries to add a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Then: Command should fail
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when adding record without columns")
		}
	})

	t.Run("must not create child without valid parent", func(t *testing.T) {
		// Given: Stash exists but parent does not
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User tries to create child with invalid parent
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", "inv-nonexistent"})
		rootCmd.Execute()

		// Then: Command should fail with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("must not accept empty primary value", func(t *testing.T) {
		// Given: Stash exists with columns
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User tries to add empty value
		rootCmd.SetArgs([]string{"add", ""})
		rootCmd.Execute()

		// Then: Command should fail
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for empty primary value")
		}
	})
}
