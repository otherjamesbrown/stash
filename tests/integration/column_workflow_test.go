package integration

import (
	"strings"
	"testing"

	"github.com/user/stash/tests/testutil"
)

// TestColumnWorkflow tests the complete column management workflow:
// add columns -> add records with fields -> list columns -> describe -> rename -> drop
//
// Use Cases Covered:
// - UC-COL-001: Add Column
// - UC-COL-002: List Columns
// - UC-COL-003: Describe Column
// - UC-COL-004: Rename Column
// - UC-COL-005: Drop Column
func TestColumnWorkflow(t *testing.T) {
	t.Run("complete column workflow", func(t *testing.T) {
		// Setup: Create stash
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		// ===== PHASE 1: Add Columns =====
		// UC-COL-001: Add Column
		t.Log("Phase 1: Adding columns")

		// Add single column
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Name")

		// Add multiple columns at once
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price", "Category", "Stock")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Price")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Category")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Stock")

		// Add column with description
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Notes", "--desc", "Additional notes about the product")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Notes")

		// ===== PHASE 2: Add Records with Fields =====
		// UC-REC-001: Add Record
		t.Log("Phase 2: Adding records with fields")

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics", "--set", "Stock=50")
		laptopID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29", "--set", "Category=electronics", "--set", "Stock=200")
		mouseID := strings.TrimSpace(result.Stdout)

		// Verify fields were set
		testutil.AssertRecordField(t, tmpDir, "inventory", laptopID, "Name", "Laptop")
		testutil.AssertRecordField(t, tmpDir, "inventory", laptopID, "Price", "999")
		testutil.AssertRecordField(t, tmpDir, "inventory", laptopID, "Category", "electronics")
		testutil.AssertRecordField(t, tmpDir, "inventory", laptopID, "Stock", "50")

		testutil.AssertRecordField(t, tmpDir, "inventory", mouseID, "Name", "Mouse")
		testutil.AssertRecordField(t, tmpDir, "inventory", mouseID, "Price", "29")

		// ===== PHASE 3: List Columns =====
		// UC-COL-002: List Columns
		t.Log("Phase 3: Listing columns")

		result = testutil.MustSucceedInDir(t, tmpDir, "column", "list")
		testutil.AssertContains(t, result, "Name")
		testutil.AssertContains(t, result, "Price")
		testutil.AssertContains(t, result, "Category")
		testutil.AssertContains(t, result, "Stock")
		testutil.AssertContains(t, result, "Notes")

		// List with JSON output
		result = testutil.MustSucceedInDir(t, tmpDir, "column", "list", "--json")
		columns := testutil.ParseJSONOutput(t, result.Stdout)
		if len(columns) != 5 {
			t.Errorf("expected 5 columns, got %d", len(columns))
		}

		// ===== PHASE 4: Describe Column =====
		// UC-COL-003: Describe Column
		t.Log("Phase 4: Describing columns")

		// Update description
		testutil.MustSucceedInDir(t, tmpDir, "column", "describe", "Price", "Price in USD")

		// Verify description appears in list
		result = testutil.MustSucceedInDir(t, tmpDir, "column", "list")
		testutil.AssertContains(t, result, "Price in USD")

		// Verify all columns still exist after workflow
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Name")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Price")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Category")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Stock")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Notes")
	})
}

// TestColumnAdd tests column addition scenarios
// UC-COL-001: Add Column
func TestColumnAdd(t *testing.T) {
	t.Run("add single column", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Name")
	})

	t.Run("add multiple columns", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "Price", "Category")

		testutil.AssertColumnExists(t, tmpDir, "inventory", "Name")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Price")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Category")
	})

	t.Run("add column with description", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price", "--desc", "Price in USD")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Price")

		// Verify description in config
		config := testutil.ReadConfig(t, tmpDir, "inventory")
		columns := testutil.GetColumns(config)
		found := false
		for _, col := range columns {
			if testutil.GetField(col, "name") == "Price" {
				if testutil.GetField(col, "desc") == "Price in USD" {
					found = true
				}
				break
			}
		}
		if !found {
			t.Error("expected column with description")
		}
	})

	t.Run("reject duplicate column", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name")

		result := testutil.MustFailInDir(t, tmpDir, "column", "add", "Name")
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for duplicate column")
		}
	})

	t.Run("case-insensitive duplicate detection", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name")

		// Should reject "name" since "Name" already exists
		result := testutil.MustFailInDir(t, tmpDir, "column", "add", "name")
		if result.ExitCode == 0 {
			t.Error("expected case-insensitive duplicate detection")
		}
	})

	t.Run("reject reserved column names", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		// Try to add reserved column names
		reservedNames := []string{"_id", "_hash", "_created_at", "_updated_at", "_deleted"}
		for _, name := range reservedNames {
			result := testutil.RunStashInDir(t, tmpDir, "column", "add", name)
			if result.ExitCode != 2 {
				t.Errorf("expected exit code 2 for reserved name '%s', got %d", name, result.ExitCode)
			}
		}
	})
}

// TestColumnList tests column listing functionality
// UC-COL-002: List Columns
func TestColumnList(t *testing.T) {
	t.Run("list columns with stats", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})

		// Add records to have stats
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29", "--set", "Category=electronics")

		// List columns
		result := testutil.MustSucceedInDir(t, tmpDir, "column", "list")
		testutil.AssertContains(t, result, "Name")
		testutil.AssertContains(t, result, "Price")
		testutil.AssertContains(t, result, "Category")
	})

	t.Run("list columns JSON output", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Add record
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")

		// Get JSON output
		result := testutil.MustSucceedInDir(t, tmpDir, "column", "list", "--json")
		columns := testutil.ParseJSONOutput(t, result.Stdout)

		if len(columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(columns))
		}

		// Verify each column has required fields
		for _, col := range columns {
			if testutil.GetField(col, "name") == "" {
				t.Error("expected name in column output")
			}
		}
	})
}

// TestColumnDescribe tests column description functionality
// UC-COL-003: Describe Column
func TestColumnDescribe(t *testing.T) {
	t.Run("set description", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Price"})

		testutil.MustSucceedInDir(t, tmpDir, "column", "describe", "Price", "Price in USD")

		// Verify in list
		result := testutil.MustSucceedInDir(t, tmpDir, "column", "list")
		testutil.AssertContains(t, result, "Price in USD")
	})

	t.Run("update existing description", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		// Add column with initial description
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price", "--desc", "Cost")

		// Update description
		testutil.MustSucceedInDir(t, tmpDir, "column", "describe", "Price", "Price in USD, excluding tax")

		// Verify updated
		result := testutil.MustSucceedInDir(t, tmpDir, "column", "list")
		testutil.AssertContains(t, result, "Price in USD, excluding tax")
	})

	t.Run("reject non-existent column", func(t *testing.T) {
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		result := testutil.MustFailInDir(t, tmpDir, "column", "describe", "FakeColumn", "Description")
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for non-existent column")
		}
	})
}

// NOTE: UC-COL-004 (Rename Column) and UC-COL-005 (Drop Column) are not yet implemented.
// When implemented, add tests for:
// - column rename preserving data
// - column drop with confirmation

// TestColumnAndRecordInteraction tests the interaction between columns and records
func TestColumnAndRecordInteraction(t *testing.T) {
	t.Run("adding column after records exist", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		// Add records
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		laptopID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse")
		mouseID := strings.TrimSpace(result.Stdout)

		// Add new column
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price")

		// Update existing records with new column
		testutil.MustSucceedInDir(t, tmpDir, "set", laptopID, "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "set", mouseID, "Price=29")

		// Verify
		testutil.AssertRecordField(t, tmpDir, "inventory", laptopID, "Price", "999")
		testutil.AssertRecordField(t, tmpDir, "inventory", mouseID, "Price", "29")
	})

	t.Run("filtering records by column value", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Category=electronics")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Category=electronics")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Desk", "--set", "Category=furniture")

		// Filter by category
		result := testutil.MustSucceedInDir(t, tmpDir, "list", "--where", "Category = 'electronics'", "--json")
		records := testutil.ParseJSONOutput(t, result.Stdout)
		if len(records) != 2 {
			t.Errorf("expected 2 electronics records, got %d", len(records))
		}

		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--where", "Category = 'furniture'", "--json")
		records = testutil.ParseJSONOutput(t, result.Stdout)
		if len(records) != 1 {
			t.Errorf("expected 1 furniture record, got %d", len(records))
		}
	})

	t.Run("selecting specific columns in list", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics")

		// Select specific columns
		result := testutil.MustSucceedInDir(t, tmpDir, "list", "--columns", "_id,Name,Price")
		testutil.AssertContains(t, result, "Name")
		testutil.AssertContains(t, result, "Price")
	})
}
