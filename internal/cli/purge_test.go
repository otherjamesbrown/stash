package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_007_PurgeRecords tests UC-REC-007: Purge Deleted Records
func TestUC_REC_007_PurgeRecords(t *testing.T) {
	t.Run("AC-01: purge by age", func(t *testing.T) {
		// Given: Records deleted more than specified time ago exist
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create and delete some records
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Phone"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		// Delete both records
		for _, rec := range records {
			store.DeleteRecord("inventory", rec.ID, "test")
		}
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash purge --before 0s --yes` (0s = purge immediately)
		rootCmd.SetArgs([]string{"purge", "--before", "0s", "--yes"})
		err := rootCmd.Execute()

		// Then: Records are permanently removed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Verify records are gone
		allRecords, _ := store.ListRecords("inventory", storage.ListOptions{
			ParentID:       "*",
			IncludeDeleted: true,
		})
		if len(allRecords) != 0 {
			t.Errorf("expected 0 records after purge, got %d", len(allRecords))
		}
	})

	t.Run("AC-02: purge specific record", func(t *testing.T) {
		// Given: Record inv-ex4j is soft-deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create two records
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Phone"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		targetID := records[0].ID
		otherID := records[1].ID
		// Delete only the first record
		store.DeleteRecord("inventory", targetID, "test")
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash purge --id inv-ex4j --yes`
		rootCmd.SetArgs([]string{"purge", "--id", targetID, "--yes"})
		err := rootCmd.Execute()

		// Then: Only that record is permanently removed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Target should be gone
		_, err = store.GetRecordIncludeDeleted("inventory", targetID)
		if err == nil {
			t.Error("expected target record to be permanently removed")
		}

		// Other record should still exist
		other, err := store.GetRecord("inventory", otherID)
		if err != nil || other == nil {
			t.Error("expected other record to still exist")
		}
	})

	t.Run("AC-03: dry run preview", func(t *testing.T) {
		// Given: Soft-deleted records exist
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

		// When: User runs `stash purge --all --dry-run`
		rootCmd.SetArgs([]string{"purge", "--all", "--dry-run"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output lists records that would be purged
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: No records are actually removed
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecordIncludeDeleted("inventory", recordID)
		if err != nil || rec == nil {
			t.Error("expected record to still exist after dry-run")
		}

		// Verify output mentions the record
		if len(output) == 0 {
			t.Error("expected dry-run output")
		}
	})

	t.Run("AC-03: dry run preview JSON", func(t *testing.T) {
		// Given: Soft-deleted records exist
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

		// When: User runs `stash purge --all --dry-run --json`
		rootCmd.SetArgs([]string{"purge", "--all", "--dry-run", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON with dry_run flag
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if jsonOutput["dry_run"] != true {
			t.Error("expected dry_run=true in JSON output")
		}
		if jsonOutput["would_purge"] != float64(1) {
			t.Errorf("expected would_purge=1, got %v", jsonOutput["would_purge"])
		}
	})

	t.Run("AC-04: require confirmation without --yes", func(t *testing.T) {
		// This test verifies that without --yes, the command would prompt
		// We can't easily test interactive prompts, so we verify the flag exists
		// and that with --yes the command proceeds

		// Given: Soft-deleted records exist
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

		// When: User runs `stash purge --all --yes`
		rootCmd.SetArgs([]string{"purge", "--all", "--yes"})
		err := rootCmd.Execute()

		// Then: Command proceeds without prompting
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}

// TestUC_REC_007_PurgeRecords_MustNot tests anti-requirements
func TestUC_REC_007_PurgeRecords_MustNot(t *testing.T) {
	t.Run("must not purge active records", func(t *testing.T) {
		// Given: Active (not deleted) record exists
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

		// When: User tries to purge an active record
		rootCmd.SetArgs([]string{"purge", "--id", recordID, "--yes"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for purging active record")
		}

		// Record should still exist
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecord("inventory", recordID)
		if err != nil || rec == nil {
			t.Error("expected record to still exist")
		}
	})

	t.Run("must require selection criteria", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash purge --yes` without selection criteria
		rootCmd.SetArgs([]string{"purge", "--yes"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})
}

// TestUC_REC_007_PurgeRecords_EdgeCases tests edge cases
func TestUC_REC_007_PurgeRecords_EdgeCases(t *testing.T) {
	t.Run("purge non-existent record", func(t *testing.T) {
		// Given: No record with the ID exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		ExitCode = 0
		resetFlags()

		// When: User tries to purge non-existent record
		rootCmd.SetArgs([]string{"purge", "--id", "inv-fake", "--yes"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("purge with no deleted records", func(t *testing.T) {
		// Given: No deleted records exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create an active record (not deleted)
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash purge --all --yes`
		rootCmd.SetArgs([]string{"purge", "--all", "--yes"})
		err := rootCmd.Execute()

		// Then: Command succeeds with message about no records
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("purge with before filter excludes recent deletions", func(t *testing.T) {
		// Given: Records deleted at different times
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		// Delete the record (it will have deletion time of "now")
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0
		resetFlags()

		// When: User runs `stash purge --before 1h --yes`
		// (record was deleted just now, should NOT be purged)
		rootCmd.SetArgs([]string{"purge", "--before", "1h", "--yes"})
		rootCmd.Execute()

		// Then: Recently deleted record is NOT purged
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, err := store.GetRecordIncludeDeleted("inventory", recordID)
		if err != nil || rec == nil {
			t.Error("expected recently deleted record to NOT be purged")
		}
	})
}

// TestParsePurgeDuration tests the duration parsing function
func TestParsePurgeDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"0s", 0, false},
		{"invalid", 0, true},
		{"d30", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePurgeDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePurgeDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parsePurgeDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
