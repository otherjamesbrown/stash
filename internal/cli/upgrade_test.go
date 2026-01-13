package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeStatus(t *testing.T) {
	t.Run("shows not recorded when no metadata", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run upgrade status
		rootCmd.SetArgs([]string{"upgrade", "status"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, "not recorded") {
			t.Errorf("expected 'not recorded' in output, got: %s", output)
		}

		// Verify no metadata file created yet (status doesn't create it)
		metadataPath := filepath.Join(tempDir, ".stash", "metadata.json")
		if _, err := os.Stat(metadataPath); err == nil {
			t.Error("metadata.json should not exist after status")
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run upgrade status --json
		rootCmd.SetArgs([]string{"upgrade", "status", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["current_version"] == nil {
			t.Error("expected current_version in JSON")
		}
		if result["schema_version"] == nil {
			t.Error("expected schema_version in JSON")
		}
	})
}

func TestUpgradeAck(t *testing.T) {
	t.Run("creates metadata file", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Run upgrade ack
		rootCmd.SetArgs([]string{"upgrade", "ack"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify metadata file created
		metadataPath := filepath.Join(tempDir, ".stash", "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			t.Fatalf("failed to read metadata.json: %v", err)
		}

		var metadata StashMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			t.Fatalf("failed to parse metadata.json: %v", err)
		}

		if metadata.LastStashVersion == "" {
			t.Error("expected LastStashVersion to be set")
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run upgrade ack --json
		rootCmd.SetArgs([]string{"upgrade", "ack", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["acknowledged"] != true {
			t.Errorf("expected acknowledged=true, got %v", result["acknowledged"])
		}
		if result["version"] == nil {
			t.Error("expected version in JSON")
		}
	})
}

func TestMigrate(t *testing.T) {
	t.Run("no migrations needed", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Run migrate
		rootCmd.SetArgs([]string{"migrate"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("dry-run mode", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run migrate --dry-run
		rootCmd.SetArgs([]string{"migrate", "--dry-run"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if !strings.Contains(output, "No migrations needed") {
			t.Errorf("expected 'No migrations needed' in output, got: %s", output)
		}
	})

	t.Run("JSON output format", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run migrate --json
		rootCmd.SetArgs([]string{"migrate", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["schema_version"] == nil {
			t.Error("expected schema_version in JSON")
		}
	})
}

func TestMigrateNoStash(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetFlags()

	// Don't create stash - should fail

	ExitCode = 0
	rootCmd.SetArgs([]string{"migrate"})
	rootCmd.Execute()

	if ExitCode != 1 {
		t.Errorf("expected exit code 1 for no stash, got %d", ExitCode)
	}
}
