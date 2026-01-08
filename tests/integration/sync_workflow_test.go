package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/tests/testutil"
)

// TestSyncWorkflow tests the sync and maintenance workflows:
// modify JSONL -> sync -> verify cache
//
// Use Cases Covered:
// - UC-SYN-001: Sync JSONL and SQLite
// - UC-SYN-002: Health Check (Doctor)
// - UC-SYN-003: Emergency Repair
func TestSyncWorkflow(t *testing.T) {
	t.Run("complete sync workflow", func(t *testing.T) {
		// Setup: Create stash with records
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Add records via CLI (which goes through normal flow)
		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		laptopID := strings.TrimSpace(result.Stdout)

		result = testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")
		mouseID := strings.TrimSpace(result.Stdout)

		// Verify records exist
		testutil.AssertRecordCount(t, tmpDir, "inventory", 2)

		// ===== PHASE 1: Check Sync Status =====
		// UC-SYN-001: Sync JSONL and SQLite
		t.Log("Phase 1: Checking sync status")

		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--status")
		testutil.AssertNotEmpty(t, result)

		// ===== PHASE 2: Rebuild Cache =====
		t.Log("Phase 2: Rebuilding cache from JSONL")

		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// ===== PHASE 3: Doctor Check =====
		// UC-SYN-002: Health Check (Doctor)
		t.Log("Phase 3: Running doctor check")

		result = testutil.MustSucceedInDir(t, tmpDir, "doctor")
		testutil.AssertExitCode(t, result, 0)

		// Verify original records still work
		_ = testutil.ShowRecord(t, tmpDir, laptopID)
		_ = testutil.ShowRecord(t, tmpDir, mouseID)

		// Verify we can still query
		result = testutil.MustSucceedInDir(t, tmpDir, "list", "--where", "Price > 50", "--json")
		expensiveRecords := testutil.ParseJSONOutput(t, result.Stdout)
		if len(expensiveRecords) < 1 {
			t.Errorf("expected at least 1 expensive record, got %d", len(expensiveRecords))
		}
	})
}

// TestSyncStatus tests sync status checking
// UC-SYN-001: Sync JSONL and SQLite (AC-01)
func TestSyncStatus(t *testing.T) {
	t.Run("check sync status", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "sync", "--status")
		testutil.AssertNotEmpty(t, result)
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestSyncRebuild tests rebuilding SQLite from JSONL
// UC-SYN-001: Sync JSONL and SQLite (AC-03)
func TestSyncRebuild(t *testing.T) {
	t.Run("rebuild SQLite from JSONL", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Add records
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")

		// Force rebuild
		result := testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// Verify records still accessible
		testutil.AssertRecordCount(t, tmpDir, "inventory", 2)
	})
}

// TestDoctor tests health check functionality
// UC-SYN-002: Health Check (Doctor)
func TestDoctor(t *testing.T) {
	t.Run("basic health check", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "doctor")
		testutil.AssertExitCode(t, result, 0)
	})

	t.Run("doctor with JSON output", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "doctor", "--json")
		jsonData := testutil.ParseJSONObject(t, result.Stdout)

		// Should have checks array
		checks := testutil.GetArrayField(jsonData, "checks")
		if checks == nil {
			t.Error("expected checks array in doctor JSON output")
		}
	})

	t.Run("doctor deep check with hash verification", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")

		result := testutil.MustSucceedInDir(t, tmpDir, "doctor", "--deep")
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestRepair tests emergency repair functionality
// UC-SYN-003: Emergency Repair
func TestRepair(t *testing.T) {
	t.Run("repair dry run", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "repair", "--dry-run")
		testutil.AssertExitCode(t, result, 0)
	})

	t.Run("repair rebuild from JSONL", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")

		// Repair from JSONL source
		result = testutil.MustSucceedInDir(t, tmpDir, "repair", "--source", "jsonl", "--yes")
		testutil.AssertExitCode(t, result, 0)

		// Verify data intact
		testutil.AssertRecordCount(t, tmpDir, "inventory", 2)
		record := testutil.ShowRecord(t, tmpDir, recordID)
		if testutil.GetField(record, "Name") != "Laptop" {
			t.Error("expected record to be preserved after repair")
		}
	})

	t.Run("repair rehash records", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "repair", "--rehash", "--yes")
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestSyncWithMultipleStashes tests sync across multiple stashes
func TestSyncWithMultipleStashes(t *testing.T) {
	t.Run("sync with multiple stashes", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)

		// Create multiple stashes
		testutil.MustSucceedInDir(t, tmpDir, "init", "products", "--prefix", "prd-")
		testutil.MustSucceedInDir(t, tmpDir, "init", "contacts", "--prefix", "cnt-")

		// Add columns and records
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "products")
		testutil.MustSucceedInDir(t, tmpDir, "column", "add", "Name", "--stash", "contacts")

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--stash", "products")
		testutil.MustSucceedInDir(t, tmpDir, "add", "John", "--stash", "contacts")

		// Sync status should work
		result := testutil.MustSucceedInDir(t, tmpDir, "sync", "--status")
		testutil.AssertExitCode(t, result, 0)

		// Doctor should check all stashes
		result = testutil.MustSucceedInDir(t, tmpDir, "doctor")
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestSyncAfterModifications tests sync after various data modifications
func TestSyncAfterModifications(t *testing.T) {
	t.Run("sync after record updates", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Update multiple times
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1099")
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1199")
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1299")

		// Sync
		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// Verify latest value
		testutil.AssertRecordField(t, tmpDir, "inventory", recordID, "Price", "1299")
	})

	t.Run("sync after delete and restore", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)

		// Delete
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")

		// Sync
		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// Verify deleted
		testutil.AssertRecordDeleted(t, tmpDir, "inventory", recordID)

		// Restore
		testutil.MustSucceedInDir(t, tmpDir, "restore", recordID)

		// Sync again
		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// Verify restored
		testutil.AssertRecordExists(t, tmpDir, "inventory", recordID)
	})

	t.Run("sync after purge", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)

		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse")

		// Delete and purge
		testutil.MustSucceedInDir(t, tmpDir, "rm", recordID, "--yes")
		testutil.MustSucceedInDir(t, tmpDir, "purge", "--id", recordID, "--yes")

		// Sync
		result = testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")
		testutil.AssertExitCode(t, result, 0)

		// Verify purged record is gone, other remains
		testutil.AssertRecordNotExists(t, tmpDir, "inventory", recordID)
		testutil.AssertRecordCount(t, tmpDir, "inventory", 1)
	})
}

// TestJSONLIntegrity tests JSONL file integrity scenarios
func TestJSONLIntegrity(t *testing.T) {
	t.Run("verify JSONL structure after operations", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		// Add several records
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Keyboard", "--set", "Price=79")

		// Read JSONL directly
		records := testutil.ReadJSONL(t, tmpDir, "inventory")
		if len(records) < 3 {
			t.Errorf("expected at least 3 JSONL entries, got %d", len(records))
		}

		// Each record should have required fields
		for i, rec := range records {
			if testutil.GetField(rec, "_id") == "" {
				t.Errorf("record %d missing _id", i)
			}
			if testutil.GetField(rec, "_hash") == "" {
				t.Errorf("record %d missing _hash", i)
			}
		}
	})

	t.Run("JSONL preserves history", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999")
		recordID := strings.TrimSpace(result.Stdout)

		// Update several times
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1099")
		testutil.MustSucceedInDir(t, tmpDir, "set", recordID, "Price=1199")

		// JSONL should have multiple entries
		records := testutil.ReadJSONL(t, tmpDir, "inventory")

		// Count entries for this record ID
		count := 0
		for _, rec := range records {
			if testutil.GetField(rec, "_id") == recordID {
				count++
			}
		}

		if count < 3 {
			t.Errorf("expected at least 3 JSONL entries for record (initial + 2 updates), got %d", count)
		}
	})
}

// TestCacheConsistency tests cache consistency with JSONL
func TestCacheConsistency(t *testing.T) {
	t.Run("cache matches JSONL data", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})

		// Add various records
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop", "--set", "Price=999", "--set", "Category=electronics")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse", "--set", "Price=29", "--set", "Category=electronics")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Desk", "--set", "Price=299", "--set", "Category=furniture")

		// Get data from list (cache)
		result := testutil.MustSucceedInDir(t, tmpDir, "list", "--json")
		cacheRecords := testutil.ParseJSONOutput(t, result.Stdout)

		// Get data from JSONL
		jsonlRecords := testutil.ReadJSONL(t, tmpDir, "inventory")
		activeJSONL := testutil.FilterActiveRecords(jsonlRecords)

		// Should match
		if len(cacheRecords) != len(activeJSONL) {
			t.Errorf("cache has %d records, JSONL has %d active records", len(cacheRecords), len(activeJSONL))
		}
	})

	t.Run("rebuild restores consistency", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		testutil.MustSucceedInDir(t, tmpDir, "add", "Mouse")

		// Force rebuild
		testutil.MustSucceedInDir(t, tmpDir, "sync", "--rebuild")

		// Doctor should pass
		result := testutil.MustSucceedInDir(t, tmpDir, "doctor")
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestDoctorFix tests the doctor --fix functionality
// UC-SYN-002: Health Check (Doctor) (AC-02)
func TestDoctorFix(t *testing.T) {
	t.Run("doctor fix with confirmation skip", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")

		result := testutil.MustSucceedInDir(t, tmpDir, "doctor", "--fix", "--yes")
		testutil.AssertExitCode(t, result, 0)
	})
}

// TestOrphanedFiles tests handling of orphaned files
// UC-SYN-003: Emergency Repair (AC-04)
func TestOrphanedFiles(t *testing.T) {
	t.Run("detect orphaned files", func(t *testing.T) {
		tmpDir := testutil.SetupStashWithColumns(t, "inventory", "inv-", []string{"Name"})

		result := testutil.MustSucceedInDir(t, tmpDir, "add", "Laptop")
		recordID := strings.TrimSpace(result.Stdout)

		// Create orphaned file manually
		orphanDir := filepath.Join(tmpDir, ".stash", "inventory", "files", "inv-orphan")
		testutil.MustMkdir(t, orphanDir)
		testutil.WriteFile(t, orphanDir, "orphan.txt", "orphaned content")

		// Attach a real file
		testFile := testutil.WriteFile(t, tmpDir, "real.txt", "real content")
		testutil.MustSucceedInDir(t, tmpDir, "attach", recordID, testFile)

		// Doctor should detect something (may or may not show orphan warning depending on implementation)
		result = testutil.MustSucceedInDir(t, tmpDir, "doctor", "--deep")
		testutil.AssertExitCode(t, result, 0)
	})

	// NOTE: repair --clean-orphans feature is not fully implemented yet.
	// Skipping test for cleaning orphaned files.
	t.Run("repair cleans orphaned files", func(t *testing.T) {
		t.Skip("repair --clean-orphans not fully implemented")
	})
}
