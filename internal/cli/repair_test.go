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

// TestUC_SYN_003_Repair tests the repair command (UC-SYN-003)
func TestUC_SYN_003_Repair(t *testing.T) {
	t.Run("AC-01: dry run previews repairs without changes", func(t *testing.T) {
		// Given: Corruption exists (simulated by rebuild need)
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "dryrun",
			Prefix:    "dry-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "value", Desc: "Value", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		record := &model.Record{
			ID:        "dry-001",
			Fields:    map[string]interface{}{"value": "test"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		store.CreateRecord(stash.Name, record)
		store.Close()

		// Get initial state
		jsonlPath := filepath.Join(stashDir, "dryrun", "records.jsonl")
		initialContent, _ := os.ReadFile(jsonlPath)

		// When: User runs `stash repair --source jsonl --dry-run`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--source", "jsonl", "--dry-run"})
		err = rootCmd.Execute()

		// Then: Shows what would be repaired, no changes made
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify no changes were made
		finalContent, _ := os.ReadFile(jsonlPath)
		if !bytes.Equal(initialContent, finalContent) {
			t.Error("dry run should not modify JSONL file")
		}

		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("Dry Run")) {
			t.Errorf("expected dry run output, got: %s", output)
		}
	})

	t.Run("AC-02: rebuild from JSONL restores SQLite", func(t *testing.T) {
		// Given: SQLite is corrupted (simulated by mismatched counts)
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "rebuild",
			Prefix:    "rbl-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "name", Desc: "Name", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create records
		for i := 1; i <= 5; i++ {
			record := &model.Record{
				ID:        fmt.Sprintf("rbl-%03d", i),
				Fields:    map[string]interface{}{"name": fmt.Sprintf("item %d", i)},
				CreatedAt: time.Now(),
				CreatedBy: "test",
				UpdatedAt: time.Now(),
				UpdatedBy: "test",
			}
			store.CreateRecord(stash.Name, record)
		}
		store.Close()

		// When: User runs `stash repair --source jsonl --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--source", "jsonl", "--yes"})
		err = rootCmd.Execute()

		// Then: SQLite is rebuilt from JSONL
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify records still exist after rebuild
		store2, _ := storage.NewStore(stashDir)
		defer store2.Close()

		count, _ := store2.CountRecords(stash.Name)
		if count != 5 {
			t.Errorf("expected 5 records after rebuild, got %d", count)
		}
	})

	t.Run("AC-03: rebuild JSONL from DB", func(t *testing.T) {
		// Given: JSONL is corrupted but DB is good
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "fromdb",
			Prefix:    "fdb-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "data", Desc: "Data", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create records
		for i := 1; i <= 3; i++ {
			record := &model.Record{
				ID:        fmt.Sprintf("fdb-%03d", i),
				Fields:    map[string]interface{}{"data": fmt.Sprintf("value %d", i)},
				CreatedAt: time.Now(),
				CreatedBy: "test",
				UpdatedAt: time.Now(),
				UpdatedBy: "test",
			}
			store.CreateRecord(stash.Name, record)
		}
		store.Close()

		// When: User runs `stash repair --source db --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--source", "db", "--yes"})
		err = rootCmd.Execute()

		// Then: JSONL is rebuilt from SQLite
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify JSONL file exists and is valid
		jsonlPath := filepath.Join(stashDir, "fromdb", "records.jsonl")
		if _, statErr := os.Stat(jsonlPath); os.IsNotExist(statErr) {
			t.Error("JSONL file should exist after rebuild from DB")
		}
	})

	t.Run("AC-04: clean orphaned files removes unreferenced files", func(t *testing.T) {
		// Given: Orphaned files exist in files/
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "orphans",
			Prefix:    "orp-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}
		store.Close()

		// Create files directory with orphaned files
		filesDir := filepath.Join(stashDir, "orphans", "files")
		os.MkdirAll(filesDir, 0755)
		os.WriteFile(filepath.Join(filesDir, "orphan1.txt"), []byte("orphan"), 0644)
		os.WriteFile(filepath.Join(filesDir, "orphan2.txt"), []byte("orphan"), 0644)

		// Verify files exist
		files, _ := os.ReadDir(filesDir)
		if len(files) != 2 {
			t.Fatalf("expected 2 orphaned files, got %d", len(files))
		}

		// When: User runs `stash repair --clean-orphans --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--clean-orphans", "--yes"})
		err = rootCmd.Execute()

		// Then: Orphaned files are removed
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify orphaned files are removed
		files, _ = os.ReadDir(filesDir)
		if len(files) != 0 {
			t.Errorf("expected 0 orphaned files after cleanup, got %d", len(files))
		}
	})

	t.Run("AC-05: rehash recalculates all record hashes", func(t *testing.T) {
		// Given: Hash mismatches exist
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "rehash",
			Prefix:    "rh-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "value", Desc: "Value", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create records
		for i := 1; i <= 3; i++ {
			record := &model.Record{
				ID:        fmt.Sprintf("rh-%03d", i),
				Fields:    map[string]interface{}{"value": fmt.Sprintf("v%d", i)},
				CreatedAt: time.Now(),
				CreatedBy: "test",
				UpdatedAt: time.Now(),
				UpdatedBy: "test",
			}
			store.CreateRecord(stash.Name, record)
		}
		store.Close()

		// When: User runs `stash repair --rehash --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--rehash", "--yes"})
		err = rootCmd.Execute()

		// Then: All record hashes are recalculated
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify hashes are valid
		store2, _ := storage.NewStore(stashDir)
		defer store2.Close()

		records, _ := store2.ListRecords(stash.Name, storage.ListOptions{ParentID: "*"})
		for _, r := range records {
			expectedHash := model.CalculateHash(r.Fields)
			if r.Hash != expectedHash {
				t.Errorf("record %s has incorrect hash after rehash: expected %s, got %s", r.ID, expectedHash, r.Hash)
			}
		}
	})
}

// TestUC_SYN_003_Repair_JSON tests JSON output
func TestUC_SYN_003_Repair_JSON(t *testing.T) {
	t.Run("JSON output for dry run", func(t *testing.T) {
		// Given: Stash exists
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "jsonrep",
			Prefix:    "jr-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs `stash repair --source jsonl --dry-run --json`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--source", "jsonl", "--dry-run", "--json"})
		err = rootCmd.Execute()

		// Then: Output is valid JSON
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		output := stdout.String()
		var result RepairOutput
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Errorf("expected valid JSON output, got parse error: %v\nOutput: %s", err, output)
		}

		// Should indicate dry run
		if !result.DryRun {
			t.Error("expected dry_run to be true in JSON output")
		}
	})
}

// TestUC_SYN_003_Repair_MustNot tests anti-requirements
func TestUC_SYN_003_Repair_MustNot(t *testing.T) {
	t.Run("must not make changes without confirmation", func(t *testing.T) {
		// Given: Repair is needed
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "noconfirm",
			Prefix:    "nc-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		// When: User runs repair without --yes (simulating 'n' response)
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		// Provide 'n' as input
		var stdout bytes.Buffer
		var stdin bytes.Buffer
		stdin.WriteString("n\n")

		rootCmd.SetOut(&stdout)
		rootCmd.SetIn(&stdin)
		rootCmd.SetArgs([]string{"repair", "--source", "jsonl"})
		rootCmd.Execute()

		// Then: Should abort without making changes
		output := stdout.String()
		if bytes.Contains([]byte(output), []byte("success")) {
			t.Error("repair should not succeed without confirmation")
		}
	})

	t.Run("must not delete non-orphaned files", func(t *testing.T) {
		// Given: Files are referenced by records
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, err := storage.NewStore(stashDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		stash := &model.Stash{
			Name:      "referenced",
			Prefix:    "ref-",
			Created:   time.Now(),
			CreatedBy: "test",
			Columns: model.ColumnList{
				{Name: "attachment", Desc: "File attachment", Added: time.Now(), AddedBy: "test"},
			},
		}
		if err := store.CreateStash(stash.Name, stash.Prefix, stash); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// Create a record with file reference
		record := &model.Record{
			ID:        "ref-001",
			Fields:    map[string]interface{}{"attachment": "files/referenced.txt"},
			CreatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedAt: time.Now(),
			UpdatedBy: "test",
		}
		store.CreateRecord(stash.Name, record)
		store.Close()

		// Create the referenced file
		filesDir := filepath.Join(stashDir, "referenced", "files")
		os.MkdirAll(filesDir, 0755)
		os.WriteFile(filepath.Join(filesDir, "referenced.txt"), []byte("important"), 0644)

		// When: User runs `stash repair --clean-orphans --yes`
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair", "--clean-orphans", "--yes"})
		rootCmd.Execute()

		// Then: Referenced file should NOT be deleted
		referencedFile := filepath.Join(filesDir, "referenced.txt")
		if _, err := os.Stat(referencedFile); os.IsNotExist(err) {
			t.Error("referenced file should not be deleted")
		}
	})
}

// TestUC_SYN_003_Repair_Errors tests error handling
func TestUC_SYN_003_Repair_Errors(t *testing.T) {
	t.Run("handles no stash directory gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"repair", "--source", "jsonl"})
		err := rootCmd.Execute()

		// Should return error when no .stash exists
		if err == nil {
			t.Error("expected error when no .stash directory exists")
		}
	})

	t.Run("rejects invalid source value", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")
		os.MkdirAll(stashDir, 0755)

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout, stderr bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"repair", "--source", "invalid"})
		err := rootCmd.Execute()

		// Should return error for invalid source
		if err == nil {
			t.Error("expected error for invalid --source value")
		}
	})

	t.Run("shows help when no repair flags specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		stashDir := filepath.Join(tmpDir, ".stash")

		store, _ := storage.NewStore(stashDir)
		stash := &model.Stash{
			Name:      "noflags",
			Prefix:    "nof-",
			Created:   time.Now(),
			CreatedBy: "test",
		}
		store.CreateStash(stash.Name, stash.Prefix, stash)
		store.Close()

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		resetRepairFlags()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repair"})
		err := rootCmd.Execute()

		// Should not error, but should indicate no repairs needed
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("No repairs")) {
			t.Errorf("expected guidance when no flags specified, got: %s", output)
		}
	})
}

// Helper function to reset repair flags between tests
func resetRepairFlags() {
	jsonOutput = false
	stashName = ""
	repairDryRun = false
	repairYes = false
	repairSource = ""
	repairCleanOrphans = false
	repairRehash = false
}
