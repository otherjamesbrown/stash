package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetBackupFlags resets backup command flags
func resetBackupFlags() {
	backupOutput = ""
	backupForce = false
}

// TestBackupCommand tests the backup command
func TestBackupCommand(t *testing.T) {
	t.Run("create backup with auto-generated name", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash backup`
		rootCmd.SetArgs([]string{"backup"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Backup file is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should mention the backup file
		if !strings.Contains(output, "inventory-backup-") {
			t.Errorf("expected backup filename in output, got: %s", output)
		}

		// Find and verify backup file exists
		files, _ := filepath.Glob(filepath.Join(tempDir, "inventory-backup-*.tar.gz"))
		if len(files) == 0 {
			t.Error("expected backup file to be created")
		}
	})

	t.Run("create backup with specific name", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// When: User runs `stash backup my-backup.tar.gz`
		backupFile := filepath.Join(tempDir, "my-backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile})
		err := rootCmd.Execute()

		// Then: Backup file is created with specified name
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		if _, err := os.Stat(backupFile); os.IsNotExist(err) {
			t.Error("expected backup file to exist")
		}
	})

	t.Run("backup includes stash config and records", func(t *testing.T) {
		// Given: Stash has records
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

		// Open and inspect backup
		file, err := os.Open(backupFile)
		if err != nil {
			t.Fatalf("failed to open backup: %v", err)
		}
		defer file.Close()

		gzReader, err := gzip.NewReader(file)
		if err != nil {
			t.Fatalf("failed to decompress: %v", err)
		}
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		foundConfig := false
		foundRecords := false
		foundMetadata := false

		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed to read tar: %v", err)
			}

			if strings.HasSuffix(header.Name, "config.json") {
				foundConfig = true
			}
			if strings.HasSuffix(header.Name, "records.jsonl") {
				foundRecords = true
			}
			if header.Name == "backup.json" {
				foundMetadata = true
			}
		}

		if !foundConfig {
			t.Error("expected config.json in backup")
		}
		if !foundRecords {
			t.Error("expected records.jsonl in backup")
		}
		if !foundMetadata {
			t.Error("expected backup.json metadata in backup")
		}
	})

	t.Run("refuse to overwrite existing backup without --force", func(t *testing.T) {
		// Given: Backup file already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create existing file
		backupFile := filepath.Join(tempDir, "existing-backup.tar.gz")
		os.WriteFile(backupFile, []byte("existing content"), 0644)

		// When: User runs `stash backup existing-backup.tar.gz` (without --force)
		rootCmd.SetArgs([]string{"backup", backupFile})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when file exists")
		}
	})

	t.Run("overwrite existing backup with --force", func(t *testing.T) {
		// Given: Backup file already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Create existing file
		backupFile := filepath.Join(tempDir, "existing-backup.tar.gz")
		os.WriteFile(backupFile, []byte("existing content"), 0644)

		// When: User runs `stash backup existing-backup.tar.gz --force`
		rootCmd.SetArgs([]string{"backup", backupFile, "--force"})
		err := rootCmd.Execute()

		// Then: File is overwritten
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify it's a valid tar.gz file now
		file, err := os.Open(backupFile)
		if err != nil {
			t.Fatalf("failed to open backup: %v", err)
		}
		defer file.Close()

		if _, err := gzip.NewReader(file); err != nil {
			t.Error("expected valid gzip file after overwrite")
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetBackupFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash backup --json`
		backupFile := filepath.Join(tempDir, "backup.tar.gz")
		rootCmd.SetArgs([]string{"backup", backupFile, "--json"})
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

		if result["backup_file"] != backupFile {
			t.Errorf("expected backup_file=%s, got %v", backupFile, result["backup_file"])
		}
	})

	t.Run("no stash returns error", func(t *testing.T) {
		// Given: No stash directory
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetBackupFlags()

		// When: User runs `stash backup`
		rootCmd.SetArgs([]string{"backup"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})
}
