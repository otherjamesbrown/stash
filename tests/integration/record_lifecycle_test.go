package integration

import (
	"strings"
	"testing"

	"github.com/user/stash/tests/testutil"
)

// TestRecordLifecycle tests the complete record lifecycle:
// add -> update -> soft-delete -> restore -> purge
//
// Use Cases Covered:
// - UC-REC-001: Add Record
// - UC-REC-002: Update Record Field
// - UC-REC-003: Show Record
// - UC-REC-005: Delete Record (Soft)
// - UC-REC-006: Restore Deleted Record
// - UC-REC-007: Purge Deleted Records
func TestRecordLifecycle(t *testing.T) {
	t.Run("complete record lifecycle workflow", func(t *testing.T) {
		// Setup: Create stash with columns
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Notes"})

		// ===== PHASE 1: Add Record =====
		// UC-REC-001: Add Record
		t.Log("Phase 1: Adding record")

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Verify record was created
		testutil.AssertRecordExists(t, tmpDir, "inventory", recordID)
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Name", "Laptop")
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Price", "999")

		// ===== PHASE 2: Update Record =====
		// UC-REC-002: Update Record Field
		t.Log("Phase 2: Updating record")

		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1299")
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Price", "1299")

		// Update another field
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Notes=High performance laptop")
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Notes", "High performance laptop")

		// ===== PHASE 3: Show Record =====
		// UC-REC-003: Show Record
		t.Log("Phase 3: Showing record details")

		record := testutil.ShowRecord(t, tmpDir, recordID)
		if testutil.GetField(record, "_id") != recordID {
			t.Errorf("expected _id '%s', got '%s'", recordID, testutil.GetField(record, "_id"))
		}
		if testutil.GetField(record, "Name") != "Laptop" {
			t.Errorf("expected Name 'Laptop', got '%s'", testutil.GetField(record, "Name"))
		}
		if testutil.GetField(record, "Price") != "1299" {
			t.Errorf("expected Price '1299', got '%s'", testutil.GetField(record, "Price"))
		}

		// Verify hash was calculated
		if testutil.GetField(record, "_hash") == "" {
			t.Error("expected _hash to be set")
		}

		// ===== PHASE 4: Soft-Delete Record =====
		// UC-REC-005: Delete Record (Soft)
		t.Log("Phase 4: Soft-deleting record")

		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")

		// Record should be marked as deleted
		testutil.AssertRecordDeleted(t, tmpDir, "inventory", recordID)

		// Record should not appear in normal list
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--json")
		records := testutil.ParseJSONOutput(t, result.Stdout)
		if len(records) != 0 {
			t.Errorf("expected 0 records in list (deleted), got %d", len(records))
		}

		// Record should appear in deleted list
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--deleted", "--json")
		deletedRecords := testutil.ParseJSONOutput(t, result.Stdout)
		if len(deletedRecords) != 1 {
			t.Errorf("expected 1 deleted record, got %d", len(deletedRecords))
		}

		// ===== PHASE 5: Restore Record =====
		// UC-REC-006: Restore Deleted Record
		t.Log("Phase 5: Restoring record")

		testutil.MustSucceedInDir(t, tmpDir, "restore", recordID)

		// Record should no longer be deleted
		testutil.AssertRecordExists(t, tmpDir, "inventory", recordID)

		// Record should appear in normal list again
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--json")
		records = testutil.ParseJSONOutput(t, result.Stdout)
		if len(records) != 1 {
			t.Errorf("expected 1 record in list after restore, got %d", len(records))
		}

		// ===== PHASE 6: Purge Record =====
		// UC-REC-007: Purge Deleted Records
		t.Log("Phase 6: Purging record")

		// Delete again for purging
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")
		testutil.AssertRecordDeleted(t, tmpDir, "inventory", recordID)

		// Purge the specific record
		testutil.MustSucceedInDir(t, tmpDir, "purge", "--id", recordID, "--yes")

		// Record should be completely gone
		testutil.AssertRecordNotExists(t, tmpDir, "inventory", recordID)
	})
}

// TestRecordHierarchy tests parent-child record relationships
func TestRecordHierarchy(t *testing.T) {
	t.Run("create and manage child records", func(t *testing.T) {
		// Setup
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Type"})

		// Create parent record
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		parentID := strings.TrimSpace(result.Stdout)

		// Create child records
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Charger", "--parent", parentID)
		child1ID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Bag", "--parent", parentID)
		child2ID := strings.TrimSpace(result.Stdout)

		// Verify child IDs are formatted correctly (parent.1, parent.2)
		expectedChild1 := parentID + ".1"
		expectedChild2 := parentID + ".2"
		if child1ID != expectedChild1 {
			t.Errorf("expected child1 ID '%s', got '%s'", expectedChild1, child1ID)
		}
		if child2ID != expectedChild2 {
			t.Errorf("expected child2 ID '%s', got '%s'", expectedChild2, child2ID)
		}

		// Verify parent record shows children
		parentRecord := testutil.ShowRecord(t, tmpDir, parentID)
		children := testutil.GetArrayField(parentRecord, "_children")
		if len(children) != 2 {
			t.Errorf("expected 2 children, got %d", len(children))
		}

		// List children
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--parent", parentID, "--json")
		childRecords := testutil.ParseJSONOutput(t, result.Stdout)
		if len(childRecords) != 2 {
			t.Errorf("expected 2 child records, got %d", len(childRecords))
		}
	})

	t.Run("cascade delete parent with children", func(t *testing.T) {
		// Setup
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create parent with children
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		parentID := strings.TrimSpace(result.Stdout)

		testutil.MustSucceedInDir(t, tmpDir, "add", "Charger", "--parent", parentID)
		testutil.MustSucceedInDir(t, tmpDir, "add", "Bag", "--parent", parentID)

		// Delete parent without cascade should fail
		result = testutil.RunStashInDir(t, tmpDir, "rm", parentID, "--yes")
		if result.ExitCode == 0 {
			t.Error("expected delete without cascade to fail when children exist")
		}

		// Delete with cascade should succeed
		testutil.MustSucceedInDir(t, tmpDir, "rm", parentID, "--cascade", "--yes")

		// Parent should be deleted
		testutil.AssertRecordDeleted(t, tmpDir, "inventory", parentID)

		// No active records should remain
		testutil.AssertRecordCount(t, tmpDir, "inventory", 0)
	})

	t.Run("cascade restore parent with children", func(t *testing.T) {
		// Setup
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create parent with children
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		parentID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Charger", "--parent", parentID)
		child1ID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Bag", "--parent", parentID)
		child2ID := strings.TrimSpace(result.Stdout)

		// Delete all with cascade
		testutil.MustSucceedInDir(t, tmpDir, "rm", parentID, "--cascade", "--yes")

		// Restore with cascade
		testutil.MustSucceedInDir(t, tmpDir, "restore", parentID, "--cascade")

		// All should be restored
		testutil.AssertRecordExists(t, tmpDir, "inventory", parentID)
		testutil.AssertRecordExists(t, tmpDir, "inventory", child1ID)
		testutil.AssertRecordExists(t, tmpDir, "inventory", child2ID)
	})
}

// TestRecordUpdate tests various update scenarios
// UC-REC-002: Update Record Field
func TestRecordUpdate(t *testing.T) {
	t.Run("update single field", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Update
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1299")

		// Verify
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Price", "1299")

		// Show should reflect update
		record := testutil.ShowRecord(t, tmpDir, recordID)
		if testutil.GetField(record, "Price") != "1299" {
			t.Errorf("expected Price '1299', got '%s'", testutil.GetField(record, "Price"))
		}
	})

	t.Run("clear field with empty value", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Notes=Important note")
		recordID := strings.TrimSpace(result.Stdout)

		// Clear the field
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Notes=")

		// Verify field is cleared
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Notes", "")
	})

	t.Run("reject update to non-existent column", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)

		// Try to update non-existent column
		result = testutil.MustFailInDir(t, tmpDir, "set", recordID, "FakeColumn=value")
		testutil.AssertExitCode(t, result, 1)
	})

	t.Run("reject update to non-existent record", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustFailInDir(t, tmpDir, "set", "inv-fake", "Price=100")
		testutil.AssertExitCode(t, result, 4)
	})

	t.Run("reject update to deleted record", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Delete the record
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")

		// Try to update deleted record
		result = testutil.MustFailInDir(t, tmpDir, "set", recordID, "Price=1299")
		if result.ExitCode == 0 {
			t.Error("expected update to deleted record to fail")
		}
	})
}

// TestRecordShow tests record display functionality
// UC-REC-003: Show Record
func TestRecordShow(t *testing.T) {
	t.Run("show record with all fields", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics")
		recordID := strings.TrimSpace(result.Stdout)

		// Show as JSON
		record := testutil.ShowRecord(t, tmpDir, recordID)

		// Verify all fields present
		if testutil.GetField(record, "_id") == "" {
			t.Error("expected _id to be set")
		}
		if testutil.GetField(record, "_hash") == "" {
			t.Error("expected _hash to be set")
		}
		if testutil.GetField(record, "_created_at") == "" {
			t.Error("expected _created_at to be set")
		}
		if testutil.GetField(record, "_updated_at") == "" {
			t.Error("expected _updated_at to be set")
		}
		if testutil.GetField(record, "Name") != "Laptop" {
			t.Errorf("expected Name 'Laptop', got '%s'", testutil.GetField(record, "Name"))
		}
		if testutil.GetField(record, "Price") != "999" {
			t.Errorf("expected Price '999', got '%s'", testutil.GetField(record, "Price"))
		}
		if testutil.GetField(record, "Category") != "electronics" {
			t.Errorf("expected Category 'electronics', got '%s'", testutil.GetField(record, "Category"))
		}
	})

	t.Run("show record with history", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Update to create history
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1299")
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1499")

		// Show with history
		result = testutil.MustSucceedInDir(t, tmpDir, "show", recordID, "--history")
		testutil.AssertNotEmpty(t, result)
	})
}

// TestRecordAdd tests record creation scenarios
// UC-REC-001: Add Record
func TestRecordAdd(t *testing.T) {
	t.Run("add record with primary value", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)

		if !strings.HasPrefix(recordID, "inv-") {
			t.Errorf("expected ID to start with 'inv-', got '%s'", recordID)
		}

		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Name", "Laptop")
	})

	t.Run("add record with JSON output", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--json")

		record := testutil.ParseJSONObject(t, result.Stdout)
		if testutil.GetField(record, "_id") == "" {
			t.Error("expected _id in JSON output")
		}
		if testutil.GetField(record, "_hash") == "" {
			t.Error("expected _hash in JSON output")
		}
		if testutil.GetField(record, "Name") != "Laptop" {
			t.Errorf("expected Name 'Laptop', got '%s'", testutil.GetField(record, "Name"))
		}
	})

	t.Run("trim whitespace from values", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "  Laptop  ")
		recordID := strings.TrimSpace(result.Stdout)

		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Name", "Laptop")
	})

	t.Run("reject empty primary value", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustFailInDir(t, tmpDir, "add", "")
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for empty primary value")
		}
	})

	t.Run("reject invalid parent", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustFailInDir(t, tmpDir, "add", "Charger", "--parent", "inv-fake")
		testutil.AssertExitCode(t, result, 4)
	})
}

// TestPurge tests purge functionality
// UC-REC-007: Purge Deleted Records
func TestPurge(t *testing.T) {
	t.Run("purge specific record", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create and delete record
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")

		// Purge specific record
		testutil.MustSucceedInDir(t, tmpDir, "purge", "--id", recordID, "--yes")

		// Record should be gone
		testutil.AssertRecordNotExists(t, tmpDir, "inventory", recordID)
	})

	t.Run("purge all deleted records", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create and delete multiple records
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		id1 := strings.TrimSpace(result.Stdout)
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse")
		id2 := strings.TrimSpace(result.Stdout)
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Keyboard")
		id3 := strings.TrimSpace(result.Stdout)

		// Keep one record active
		testutil.MustSucceedInDir(t, tmpDir, "rm", id1, "--yes")
		testutil.MustSucceedInDir(t, tmpDir, "rm", id2, "--yes")

		// Purge all deleted
		testutil.MustSucceedInDir(t, tmpDir, "purge", "--all", "--yes")

		// Deleted records should be gone
		testutil.AssertRecordNotExists(t, tmpDir, "inventory", id1)
		testutil.AssertRecordNotExists(t, tmpDir, "inventory", id2)

		// Active record should still exist
		testutil.AssertRecordExists(t, tmpDir, "inventory", id3)
	})

	t.Run("purge dry run", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create and delete record
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")

		// Dry run purge
		result = testutil.MustSucceedInDir(t, tmpDir, "purge", "--all", "--dry-run")
		testutil.AssertContains(t, result, recordID)

		// Record should still exist
		testutil.AssertRecordDeleted(t, tmpDir, "inventory", recordID)
	})
}

// TestRecordHistory tests change history functionality
// UC-QRY-004: View Change History
func TestRecordHistory(t *testing.T) {
	t.Run("show history for record", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Create record with updates
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1099")
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1299")

		// Get history for record
		result = testutil.MustSucceedInDir(t, tmpDir, "history", recordID)
		testutil.AssertNotEmpty(t, result)
		testutil.AssertContains(t, result, recordID)
	})

	t.Run("show all recent history", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create multiple records
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Keyboard")

		// Get all history
		result := testutil.MustSucceedInDir(t, tmpDir, "history")
		testutil.AssertNotEmpty(t, result)
	})

	t.Run("history with JSON output", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "history", "--json")
		history := testutil.ParseJSONOutput(t, result.Stdout)
		if len(history) == 0 {
			t.Error("expected at least one history entry")
		}
	})

	t.Run("history with limit", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Create several records
		for i := 0; i < 10; i++ {
			testutil.MustSucceedInDir(t, tmpDir, "add", "Item")
		}

		// Get limited history
		result := testutil.MustSucceedInDir(t, tmpDir, "history", "--limit", "5", "--json")
		history := testutil.ParseJSONOutput(t, result.Stdout)
		if len(history) > 5 {
			t.Errorf("expected at most 5 history entries, got %d", len(history))
		}
	})
}
