// Package integration provides end-to-end integration tests for the stash CLI.
// These tests verify complete user workflows using the actual CLI commands.
package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/tests/testutil"
)

// TestStashLifecycle tests the complete stash lifecycle:
// init -> add columns -> add records -> query -> drop
//
// Use Cases Covered:
// - UC-ST-001: Initialize Stash
// - UC-COL-001: Add Column
// - UC-REC-001: Add Record
// - UC-QRY-001: List Records
// - UC-ST-002: Drop Stash
func TestStashLifecycle(t *testing.T) {
	t.Run("complete stash lifecycle workflow", func(t *testing.T) {
		// Setup: Create a temporary directory
		tmpDir := testutil.TempDir(t)

		// ===== PHASE 1: Initialize Stash =====
		// UC-ST-001: Initialize Stash
		t.Log("Phase 1: Initializing stash")

		result := testutil.MustSucceedInDir(t, tmpDir, "init", "inventory", "--prefix", "inv-")
		testutil.AssertStashInitialized(t, tmpDir, "inventory")

		// Verify config.json was created with correct metadata
		config := testutil.ReadConfig(t, tmpDir, "inventory")
		if testutil.GetField(config, "name") != "inventory" {
			t.Errorf("expected name 'inventory', got %v", testutil.GetField(config, "name"))
		}
		if testutil.GetField(config, "prefix") != "inv-" {
			t.Errorf("expected prefix 'inv-', got %v", testutil.GetField(config, "prefix"))
		}

		// ===== PHASE 2: Add Columns =====
		// UC-COL-001: Add Column
		t.Log("Phase 2: Adding columns")

		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--desc", "Product name")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price", "--desc", "Price in USD")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Category", "--desc", "Product category")

		testutil.AssertColumnExists(t, tmpDir, "inventory", "Name")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Price")
		testutil.AssertColumnExists(t, tmpDir, "inventory", "Category")

		// ===== PHASE 3: Add Records =====
		// UC-REC-001: Add Record
		t.Log("Phase 3: Adding records")

		// Add first record
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics")
		laptopID := strings.TrimSpace(result.Stdout)
		if !strings.HasPrefix(laptopID, "inv-") {
			t.Errorf("expected record ID to start with 'inv-', got %s", laptopID)
		}

		// Add second record
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29", "--set", "Category=electronics")
		mouseID := strings.TrimSpace(result.Stdout)

		// Add third record in different category
		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Desk", "--set", "Price=299", "--set", "Category=furniture")
		deskID := strings.TrimSpace(result.Stdout)

		// Verify records exist
		testutil.AssertRecordCount(t, tmpDir, "inventory", 3)
		testutil.AssertRecordExists(t, tmpDir, "inventory", laptopID)
		testutil.AssertRecordExists(t, tmpDir, "inventory", mouseID)
		testutil.AssertRecordExists(t, tmpDir, "inventory", deskID)

		// ===== PHASE 4: Query Records =====
		// UC-QRY-001: List Records
		t.Log("Phase 4: Querying records")

		// List all records
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--json")
		records := testutil.ParseJSONOutput(t, result.Stdout)
		if len(records) != 3 {
			t.Errorf("expected 3 records in list, got %d", len(records))
		}

		// Query with filter
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--where", "Category = 'electronics'", "--json")
		filteredRecords := testutil.ParseJSONOutput(t, result.Stdout)
		if len(filteredRecords) != 2 {
			t.Errorf("expected 2 electronics records, got %d", len(filteredRecords))
		}

		// Show specific record
		record := testutil.ShowRecord(t, tmpDir, laptopID)
		if testutil.GetField(record, "Name") != "Laptop" {
			t.Errorf("expected Name 'Laptop', got %v", testutil.GetField(record, "Name"))
		}

		// ===== PHASE 5: Drop Stash =====
		// UC-ST-002: Drop Stash
		t.Log("Phase 5: Dropping stash")

		testutil.MustSucceedInDir(t, tmpDir, "drop", "inventory", "--yes")

		// Verify stash is gone
		stashDir := testutil.StashDir(tmpDir, "inventory")
		testutil.AssertDirNotExists(t, stashDir)
	})
}

// TestStashLifecycle_ErrorRecovery tests error handling and recovery scenarios
func TestStashLifecycle_ErrorRecovery(t *testing.T) {
	t.Run("cannot init duplicate stash", func(t *testing.T) {
		// Setup
		tmpDir := testutil.TempDir(t)

		// Init first stash successfully
		testutil.MustSucceedInDir(t, tmpDir, "init", "inventory", "--prefix", "inv-")

		// Attempt to init duplicate should fail
		result := testutil.MustFailInDir(t, tmpDir, "init", "inventory", "--prefix", "inv-")
		testutil.AssertExitCode(t, result, 1)
	})

	t.Run("cannot add record without columns", func(t *testing.T) {
		// Setup: Create stash without columns
		tmpDir := testutil.TempDir(t)
		testutil.MustSucceedInDir(t, tmpDir, "init", "test", "--prefix", "tst-")

		// Attempt to add record should fail
		result := testutil.MustFailInDir(t, tmpDir, "add", "Test")
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code when adding record without columns")
		}
	})

	t.Run("cannot drop non-existent stash", func(t *testing.T) {
		// Setup
		tmpDir := testutil.TempDir(t)
		testutil.MustSucceedInDir(t, tmpDir, "init", "test", "--prefix", "tst-")

		// Attempt to drop non-existent stash should fail
		result := testutil.MustFailInDir(t, tmpDir, "drop", "nonexistent", "--yes")
		testutil.AssertExitCode(t, result, 3)
	})

	t.Run("reject invalid prefix", func(t *testing.T) {
		// Setup
		tmpDir := testutil.TempDir(t)

		// Prefix too short (needs 3-5 chars: 2-4 letters + dash)
		result := testutil.MustFailInDir(t, tmpDir, "init", "test", "--prefix", "x")
		testutil.AssertExitCode(t, result, 2)
	})
}

// TestStashLifecycle_MultipleStashes tests managing multiple stashes
func TestStashLifecycle_MultipleStashes(t *testing.T) {
	t.Run("manage multiple stashes independently", func(t *testing.T) {
		// Setup
		tmpDir := testutil.TempDir(t)

		// Create two stashes
		testutil.MustSucceedInDir(t, tmpDir, "init", "products", "--prefix", "prd-")
		testutil.MustSucceedInDir(t, tmpDir, "init", "contacts", "--prefix", "cnt-")

		// Add columns to each
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "products")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "contacts")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Price", "--stash", "products")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Email", "--stash", "contacts")

		// Add records to each
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--stash", "products")
		productID := strings.TrimSpace(result.Stdout)
		if !strings.HasPrefix(productID, "prd-") {
			t.Errorf("expected product ID to start with 'prd-', got %s", productID)
		}

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "John", "--stash", "contacts")
		contactID := strings.TrimSpace(result.Stdout)
		if !strings.HasPrefix(contactID, "cnt-") {
			t.Errorf("expected contact ID to start with 'cnt-', got %s", contactID)
		}

		// Verify records are in correct stashes
		testutil.AssertRecordCount(t, tmpDir, "products", 1)
		testutil.AssertRecordCount(t, tmpDir, "contacts", 1)

		// Drop one stash, other should remain
		testutil.MustSucceedInDir(t, tmpDir, "drop", "products", "--yes")
		testutil.AssertDirNotExists(t, testutil.StashDir(tmpDir, "products"))
		testutil.AssertStashInitialized(t, tmpDir, "contacts")

		// Remaining stash should still work
		testutil.MustSucceedInDir(t, tmpDir, "add", "Jane", "--stash", "contacts")
		testutil.AssertRecordCount(t, tmpDir, "contacts", 2)
	})
}

// TestStashInfo tests the info command
// UC-ST-003: Show Stash Info
func TestStashInfo(t *testing.T) {
	t.Run("show stash info with stats", func(t *testing.T) {
		// Setup: Create stash with records
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Add some records
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")

		// Get info
		result := testutil.MustSucceedInDir(t, tmpDir, "info")
		testutil.AssertContains(t, result, "inventory")
	})

	t.Run("info with JSON output", func(t *testing.T) {
		// Setup
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Test")

		// Get info as JSON
		result := testutil.MustSucceedInDir(t, tmpDir, "info", "--json")
		jsonData := testutil.ParseJSONObject(t, result.Stdout)

		// Should have stashes array
		stashes := testutil.GetArrayField(jsonData, "stashes")
		if stashes == nil {
			t.Error("expected stashes array in JSON output")
		}
	})
}

// TestStashPrime tests the prime command
// UC-ST-005: Generate Context for Agent
func TestStashPrime(t *testing.T) {
	t.Run("generate context for agent", func(t *testing.T) {
		// Setup: Create stash with columns and records
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29", "--set", "Category=electronics")

		// Get prime context
		result := testutil.MustSucceedInDir(t, tmpDir, "prime")

		// Should include column info
		testutil.AssertContains(t, result, "Name")
		testutil.AssertContains(t, result, "Price")
		testutil.AssertContains(t, result, "Category")
	})

	t.Run("prime for specific stash", func(t *testing.T) {
		// Setup: Create multiple stashes
		tmpDir := testutil.TempDir(t)

		testutil.MustSucceedInDir(t, tmpDir, "init", "products", "--prefix", "prd-")
		testutil.MustSucceedInDir(t, tmpDir, "init", "contacts", "--prefix", "cnt-")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "products")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "contacts")

		// Prime for specific stash
		result := testutil.MustSucceedInDir(t, tmpDir, "prime", "--stash", "products")
		testutil.AssertContains(t, result, "products")
	})
}

// TestStashOnboard tests the onboard command
// UC-ST-004: Generate Onboarding Snippet
func TestStashOnboard(t *testing.T) {
	t.Run("generate onboarding snippet", func(t *testing.T) {
		// Setup
		tmpDir := testutil.SetupStash(t, "inventory", "inv-")

		// Get onboard snippet
		result := testutil.MustSucceedInDir(t, tmpDir, "onboard")

		// Should output markdown
		testutil.AssertContains(t, result, "stash")
	})
}

// TestStashFilesDirectory verifies files directory is created
func TestStashFilesDirectory(t *testing.T) {
	t.Run("init creates files directory", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)
		testutil.MustSucceedInDir(t, tmpDir, "init", "inventory", "--prefix", "inv-")

		filesDir := filepath.Join(tmpDir, ".stash", "inventory", "files")
		testutil.AssertDirExists(t, filesDir)
	})
}
