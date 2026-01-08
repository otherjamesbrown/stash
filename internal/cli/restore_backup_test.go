package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// resetRestoreBackupFlags resets restore-backup command flags
func resetRestoreBackupFlags() {
	restoreBackupForce = false
	restoreBackupConfirm = false
	restoreBackupDryRun = false
}

// TestRestoreBackupCommand tests the restore-backup command
func TestRestoreBackupCommand(t *testing.T) {
	t.Run("restore backup with --confirm", func(t *testing.T) {
		// Given: A backup file exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create some records
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create backup
		backupFile := filepath.Join(tempDir, "test-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Delete the stash
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetRestoreBackupFlags()

		// When: User runs `stash restore-backup test-backup.tar.gz --confirm`
		rootCmd.SetArgs([]string{"restore-backup", backupFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Stash is restored
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify records were restored
		store, err := storage.NewStore(filepath.Join(tempDir, ".stash"))
		if err != nil {
			t.Fatalf("failed to open store: %v", err)
		}
		defer store.Close()

		records, err := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if err != nil {
			t.Fatalf("failed to list records: %v", err)
		}
		if len(records) != 2 {
			t.Errorf("expected 2 records after restore, got %d", len(records))
		}
	})

	t.Run("dry run preview", func(t *testing.T) {
		// Given: A backup file exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create backup
		backupFile := filepath.Join(tempDir, "test-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Delete the stash
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetRestoreBackupFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash restore-backup test-backup.tar.gz --dry-run`
		rootCmd.SetArgs([]string{"restore-backup", backupFile, "--dry-run"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows what would be restored
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: No changes are made
		store, err := storage.NewStore(filepath.Join(tempDir, ".stash"))
		if err == nil {
			stashes, _ := store.ListStashes()
			store.Close()
			if len(stashes) > 0 {
				t.Error("expected no stash to exist after dry run")
			}
		}

		// Output should mention dry run
		if !strings.Contains(output, "Dry run") && !strings.Contains(output, "dry run") {
			t.Errorf("expected 'dry run' in output, got: %s", output)
		}
	})

	t.Run("restore over existing stash with --force", func(t *testing.T) {
		// Given: A backup file exists and stash already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create backup
		backupFile := filepath.Join(tempDir, "test-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Add another record (so stash differs from backup)
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetRestoreBackupFlags()

		// When: User runs `stash restore-backup test-backup.tar.gz --force --confirm`
		rootCmd.SetArgs([]string{"restore-backup", backupFile, "--force", "--confirm"})
		err := rootCmd.Execute()

		// Then: Stash is overwritten with backup
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify only original record exists (Mouse should be gone)
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 1 {
			t.Errorf("expected 1 record after restore, got %d", len(records))
		}
	})

	t.Run("refuse to overwrite existing stash without --force", func(t *testing.T) {
		// Given: A backup file exists and stash already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create backup
		backupFile := filepath.Join(tempDir, "test-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetRestoreBackupFlags()

		// When: User runs `stash restore-backup test-backup.tar.gz --confirm` (without --force)
		rootCmd.SetArgs([]string{"restore-backup", backupFile, "--confirm"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when stash exists")
		}
	})

	t.Run("backup file not found returns error", func(t *testing.T) {
		// Given: No backup file
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetRestoreBackupFlags()

		// When: User runs `stash restore-backup nonexistent.tar.gz --confirm`
		rootCmd.SetArgs([]string{"restore-backup", "nonexistent.tar.gz", "--confirm"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for missing file")
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		// Given: A backup file exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create backup
		backupFile := filepath.Join(tempDir, "test-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Delete the stash
		rootCmd.SetArgs([]string{"drop", "inventory", "--yes"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetRestoreBackupFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash restore-backup test-backup.tar.gz --confirm --json`
		rootCmd.SetArgs([]string{"restore-backup", backupFile, "--confirm", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["stash"] != "inventory" {
			t.Errorf("expected stash=inventory, got %v", result["stash"])
		}
	})

	t.Run("invalid backup file returns error", func(t *testing.T) {
		// Given: Invalid backup file
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetRestoreBackupFlags()

		// Create invalid backup file
		invalidFile := filepath.Join(tempDir, "invalid.tar.gz")
		os.WriteFile(invalidFile, []byte("not a valid gzip file"), 0644)

		// When: User runs `stash restore-backup invalid.tar.gz --confirm`
		rootCmd.SetArgs([]string{"restore-backup", invalidFile, "--confirm"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for invalid backup")
		}
	})
}
