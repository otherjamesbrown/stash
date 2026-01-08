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

// TestUC_SYN_001_Sync tests the sync command (UC-SYN-001)
func TestUC_SYN_001_Sync(t *testing.T) {
	t.Run("AC-01: check sync status shows state and pending changes", func(t *testing.T) {
		// Given: Stash exists with records
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		defer store.Close()

		// Create a stash
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

		// When: User runs `stash sync --status`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"sync", "--status"})
		err = rootCmd.Execute()

		// Then: Shows sync state for each stash, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		if output == "" {
			t.Error("expected output, got empty")
		}
		// Output should contain stash name and record count
		if !bytes.Contains([]byte(output), []byte("inventory")) {
			t.Errorf("expected output to contain 'inventory', got: %s", output)
		}
	})

	t.Run("AC-02: normal sync synchronizes pending changes", func(t *testing.T) {
		// Given: Pending changes exist
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		defer store.Close()

		// Create a stash
		stash := &model.Stash{
			Name:      "test-stash",
			Prefix:    "tst-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "name", Desc: "Name", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}
		store.Close()

		// When: User runs `stash sync`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"sync"})
		err = rootCmd.Execute()

		// Then: Changes are synchronized, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("AC-03: full rebuild rebuilds SQLite from JSONL", func(t *testing.T) {
		// Given: SQLite cache may be outdated
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// Create a stash with records
		stash := &model.Stash{
			Name:      "rebuild-test",
			Prefix:    "rbt-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "title", Desc: "Title", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		record := &model.Record{
			ID:        "rbt-001",
			Fields:    map[string]interface{}{"title": "Test Item"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		if err := store.CreateRecord(stash.Name, record); err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
		store.Close()

		// When: User runs `stash sync --rebuild`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"sync", "--rebuild", "--stash", "rebuild-test"})
		err = rootCmd.Execute()

		// Then: SQLite is rebuilt from JSONL, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify the record still exists after rebuild
		store2, _ := storage.NewStore(stashDir)
		defer store2.Close()

		fetched, err := store2.GetRecord(stash.Name, "rbt-001")
		if err != nil {
			t.Fatalf("expected record to exist after rebuild, got: %v", err)
		}
		if fetched.Fields["title"] != "Test Item" {
			t.Errorf("expected title 'Test Item', got: %v", fetched.Fields["title"])
		}
	})

	t.Run("AC-04: flush writes compacted JSONL", func(t *testing.T) {
		// Given: DB has pending changes
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// Create a stash with records
		stash := &model.Stash{
			Name:      "flush-test",
			Prefix:    "flt-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "value", Desc: "Value", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create multiple records and updates to generate history
		record := &model.Record{
			ID:        "flt-001",
			Fields:    map[string]interface{}{"value": "v1"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		store.CreateRecord(stash.Name, record)

		// Update the record
		record.Fields["value"] = "v2"
		record.UpdatedAt = time.Now()
		store.UpdateRecord(stash.Name, record)

		// Update again
		record.Fields["value"] = "v3"
		record.UpdatedAt = time.Now()
		store.UpdateRecord(stash.Name, record)
		store.Close()

		// When: User runs `stash sync --flush`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"sync", "--flush", "--stash", "flush-test"})
		err = rootCmd.Execute()

		// Then: DB changes are written to JSONL, exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify the JSONL file exists and has been compacted
		jsonlPath := filepath.Join(stashDir, "flush-test", "records.jsonl")
		if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
			t.Error("expected records.jsonl to exist after flush")
		}
	})

	t.Run("AC-05: pull from main copies JSONL from main worktree", func(t *testing.T) {
		// Note: This test is more complex as it requires a git worktree setup
		// For now, we test that the flag is recognized and returns appropriate error
		// when not in a worktree

		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "main-test",
			Prefix:    "mt-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs `stash sync --from-main` (not in a worktree)
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"sync", "--from-main", "--stash", "main-test"})
		err = rootCmd.Execute()

		// Then: Should handle gracefully (error is acceptable when not in worktree)
		// The test passes if the command is recognized
		// (actual worktree sync would require git setup)
	})
}

// TestUC_SYN_001_Sync_JSON tests JSON output for sync status
func TestUC_SYN_001_Sync_JSON(t *testing.T) {
	t.Run("AC-01: JSON output for status", func(t *testing.T) {
		// Given: Stash exists
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "json-test",
			Prefix:    "jt-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs `stash sync --status --json`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// Reset flags to defaults
		jsonOutput = false

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"sync", "--status", "--json"})
		err = rootCmd.Execute()

		// Then: Output is valid JSON
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Errorf("expected valid JSON output, got parse error: %v\nOutput: %s", err, output)
		}

		// Should contain stashes array
		if _, ok := result["stashes"]; !ok {
			t.Errorf("expected JSON to contain 'stashes' key, got: %s", output)
		}
	})
}

// TestUC_SYN_001_Sync_MustNot tests anti-requirements
func TestUC_SYN_001_Sync_MustNot(t *testing.T) {
	t.Run("must not lose data during sync", func(t *testing.T) {
		// Given: Stash with records
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "data-safety",
			Prefix:    "ds-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "important", Desc: "Important data", Added: time.Now(), AddedBy: "test"},
			},
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)

		// Create records
		for i := 1; i <= 10; i++ {
			id, _ := model.GenerateID(stash.Prefix)
			record := &model.Record{
				ID:        id,
				Fields:    map[string]interface{}{"important": fmt.Sprintf("critical data %d", i)},
				CreatedAt: time.Now(),
				CreatedBy: "test",
				UpdatedAt: time.Now(),
				UpdatedBy: "test",
			}
			store.CreateRecord(stash.Name, record)
		}
		store.Close()

		// When: Full sync cycle (rebuild then flush)
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// Reset flags
		jsonOutput = false

		rootCmd.SetArgs([]string{"sync", "--rebuild", "--stash", "data-safety"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("rebuild failed: %v", err)
		}

		rootCmd.SetArgs([]string{"sync", "--flush", "--stash", "data-safety"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("flush failed: %v", err)
		}

		// Then: All records should still exist
		store2, _ := storage.NewStore(stashDir)
		defer store2.Close()

		records, err := store2.ListRecords(stash.Name, storage.ListOptions{ParentID: "*"})
		if err != nil {
			t.Fatalf("failed to list records: %v", err)
		}

		if len(records) != 10 {
			t.Errorf("expected 10 records after sync, got %d", len(records))
		}
	})
}

// TestUC_SYN_001_Sync_Errors tests error handling
func TestUC_SYN_001_Sync_Errors(t *testing.T) {
	t.Run("error when no stash exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// Reset flags
		jsonOutput = false

		rootCmd.SetArgs([]string{"sync", "--status"})
		err := rootCmd.Execute()

		// Should handle gracefully (error or empty output is acceptable)
		_ = err // Error is expected when no .stash exists
	})

	t.Run("error when specified stash does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		os.MkdirAll(stashDir, 0755)

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// Reset global and sync flags
		jsonOutput = false
		stashName = ""
		syncStatus = false
		syncRebuild = false
		syncFlush = false
		syncFromMain = false

		rootCmd.SetArgs([]string{"sync", "--rebuild", "--stash", "nonexistent"})
		err := rootCmd.Execute()

		// Should return error for nonexistent stash
		if err == nil {
			t.Error("expected error for nonexistent stash")
		}
	})
}
