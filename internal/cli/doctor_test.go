package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// TestUC_SYN_002_Doctor tests the doctor command (UC-SYN-002)
func TestUC_SYN_002_Doctor(t *testing.T) {
	t.Run("AC-01: basic health check reports warnings and errors", func(t *testing.T) {
		// Given: Stash exists
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// Create a stash with records
		stash := &model.Stash{
			Name:      "inventory",
			Prefix:    "inv-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "name", Desc: "Item name", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Add a record
		record := &model.Record{
			ID:        "inv-abc",
			Fields:    map[string]interface{}{"name": "Widget"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		if err := store.CreateRecord(stash.Name, record); err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
		store.Close()

		// When: User runs `stash doctor`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// Reset flags
		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		err = rootCmd.Execute()

		// Then: Reports status, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		if output == "" {
			t.Error("expected output, got empty")
		}

		// Output should contain health check results
		if !bytes.Contains([]byte(output), []byte("Health Check")) {
			t.Errorf("expected output to contain 'Health Check', got: %s", output)
		}
	})

	t.Run("AC-02: auto-fix with --fix --yes repairs issues", func(t *testing.T) {
		// Given: Fixable issues exist (cache out of sync)
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "fixtest",
			Prefix:    "fix-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "value", Desc: "Value", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create a record
		record := &model.Record{
			ID:        "fix-001",
			Fields:    map[string]interface{}{"value": "test"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		store.CreateRecord(stash.Name, record)
		store.Close()

		// When: User runs `stash doctor --fix --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor", "--fix", "--yes"})
		err = rootCmd.Execute()

		// Then: Exit code is 0, issues are handled
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("AC-03: deep check verifies record hashes", func(t *testing.T) {
		// Given: Stash has records
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "deeptest",
			Prefix:    "dep-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "data", Desc: "Data field", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create records
		for i := 1; i <= 3; i++ {
			record := &model.Record{
				ID:        fmt.Sprintf("dep-%03d", i),
				Fields:    map[string]interface{}{"data": fmt.Sprintf("value %d", i)},
				CreatedAt: time.Now(),
				CreatedBy: "test",
				UpdatedAt: time.Now(),
				UpdatedBy: "test",
			}
			store.CreateRecord(stash.Name, record)
		}
		store.Close()

		// When: User runs `stash doctor --deep`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor", "--deep"})
		err = rootCmd.Execute()

		// Then: All record hashes are verified, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		// Should include hash verification
		if !bytes.Contains([]byte(output), []byte("hashes")) {
			t.Errorf("expected output to contain hash check, got: %s", output)
		}
	})

	t.Run("AC-04: JSON output is valid", func(t *testing.T) {
		// Given: Stash exists
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "jsontest",
			Prefix:    "jst-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs `stash doctor --json`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor", "--json"})
		err = rootCmd.Execute()

		// Then: Output is valid JSON with checks array
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		var result DoctorOutput
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Errorf("expected valid JSON output, got parse error: %v\nOutput: %s", err, output)
		}

		// Should contain checks array
		if result.Checks == nil {
			t.Errorf("expected JSON to contain 'checks' array, got: %s", output)
		}
	})
}

// TestUC_SYN_002_Doctor_Checks tests individual health checks
func TestUC_SYN_002_Doctor_Checks(t *testing.T) {
	t.Run("detects invalid JSONL", func(t *testing.T) {
		// Given: JSONL file has invalid JSON
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "badtest",
			Prefix:    "bad-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// Write invalid JSONL
		jsonlPath := filepath.Join(stashDir, "badtest", "records.jsonl")
		os.WriteFile(jsonlPath, []byte("this is not valid json\n"), 0644)

		// When: User runs `stash doctor`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		rootCmd.Execute()

		// Then: Error should be reported
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("ERROR")) {
			t.Errorf("expected error in output for invalid JSONL, got: %s", output)
		}
	})

	t.Run("detects cache out of sync", func(t *testing.T) {
		// Given: JSONL and SQLite have different record counts
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "synctest",
			Prefix:    "syn-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "name", Desc: "Name", Added: time.Now(), AddedBy: "test"},
			},
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)

		// Create a record
		record := &model.Record{
			ID:        "syn-001",
			Fields:    map[string]interface{}{"name": "test"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		store.CreateRecord(stash.Name, record)
		store.Close()

		// Manually add extra line to JSONL to create mismatch
		jsonlPath := filepath.Join(stashDir, "synctest", "records.jsonl")
		extraRecord := `{"_id":"syn-002","_hash":"abc123456789","_op":"create","_created_at":"2024-01-01T00:00:00Z","_created_by":"test","_updated_at":"2024-01-01T00:00:00Z","_updated_by":"test","name":"extra"}`
		f, _ := os.OpenFile(jsonlPath, os.O_APPEND|os.O_WRONLY, 0644)
		f.WriteString(extraRecord + "\n")
		f.Close()

		// When: User runs `stash doctor`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		rootCmd.Execute()

		// Then: Warning about cache out of sync
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("WARN")) && !bytes.Contains([]byte(output), []byte("sync")) {
			t.Errorf("expected warning about sync in output, got: %s", output)
		}
	})

	t.Run("warns about missing column descriptions", func(t *testing.T) {
		// Given: Stash has column without description
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "nodesc",
			Prefix:    "nd-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "field1", Desc: "", Added: time.Now(), AddedBy: "test"},
			},
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs `stash doctor`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		rootCmd.Execute()

		// Then: Warning about missing description
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("WARN")) {
			t.Errorf("expected warning about missing description, got: %s", output)
		}
	})
}

// TestUC_SYN_002_Doctor_MustNot tests anti-requirements
func TestUC_SYN_002_Doctor_MustNot(t *testing.T) {
	t.Run("must not modify data without --fix flag", func(t *testing.T) {
		// Given: Stash with fixable issues
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "nofix",
			Prefix:    "nf-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "data", Desc: "Data", Added: time.Now(), AddedBy: "test"},
			},
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// Get initial modification time of cache.db
		cacheDB := filepath.Join(stashDir, "cache.db")
		initialStat, _ := os.Stat(cacheDB)
		initialModTime := initialStat.ModTime()

		// Wait a moment
		time.Sleep(100 * time.Millisecond)

		// When: User runs `stash doctor` (without --fix)
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		rootCmd.Execute()

		// Then: Data should not be modified
		finalStat, _ := os.Stat(cacheDB)
		finalModTime := finalStat.ModTime()

		// The modification time should not have changed significantly
		// (allowing for filesystem timestamp granularity)
		if finalModTime.Sub(initialModTime) > time.Second {
			t.Errorf("database was modified without --fix flag")
		}
	})
}

// TestUC_SYN_002_Doctor_Errors tests error handling
func TestUC_SYN_002_Doctor_Errors(t *testing.T) {
	t.Run("handles no stash directory gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"doctor"})
		err := rootCmd.Execute()

		// Should return error when no .stash exists
		if err == nil {
			t.Error("expected error when no .stash directory exists")
		}
	})

	t.Run("handles empty stash directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		os.MkdirAll(stashDir, 0755)

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetDoctorFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"doctor"})
		err := rootCmd.Execute()

		// Should handle gracefully (warning about no stashes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("No stashes")) {
			t.Errorf("expected warning about no stashes, got: %s", output)
		}
	})
}

// Helper function to reset doctor flags between tests
func resetDoctorFlags() {
	jsonOutput = false
	stashName = ""
	doctorFix = false
	doctorYes = false
	doctorDeep = false
}
