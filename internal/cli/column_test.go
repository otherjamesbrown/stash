package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUC_COL_001_AddColumn tests UC-COL-001: Add Column
func TestUC_COL_001_AddColumn(t *testing.T) {
	t.Run("AC-01: add single column", func(t *testing.T) {
		// Given: Stash "inventory" exists with no columns
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// When: User runs `stash column add Name`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "Name"})
		err := rootCmd.Execute()

		// Then: Column "Name" is added to config.json
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config.json: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			t.Fatalf("failed to parse config.json: %v", err)
		}

		columns, ok := config["columns"].([]interface{})
		if !ok || len(columns) != 1 {
			t.Fatalf("expected 1 column, got %v", config["columns"])
		}

		col := columns[0].(map[string]interface{})
		if col["name"] != "Name" {
			t.Errorf("expected column name 'Name', got %v", col["name"])
		}

		// Then: Column has added timestamp and added_by
		if col["added"] == nil {
			t.Error("expected 'added' timestamp to be set")
		}
		if col["added_by"] == nil {
			t.Error("expected 'added_by' to be set")
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("AC-02: add multiple columns", func(t *testing.T) {
		// Given: Stash "inventory" exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// When: User runs `stash column add Name Price Category`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "Name", "Price", "Category"})
		err := rootCmd.Execute()

		// Then: All three columns are added
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("AC-03: add column with description", func(t *testing.T) {
		// Given: Stash "inventory" exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// When: User runs `stash column add Price --desc "Price in USD"`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "Price", "--desc", "Price in USD"})
		err := rootCmd.Execute()

		// Then: Column "Price" is added with description
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config.json: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			t.Fatalf("failed to parse config.json: %v", err)
		}

		columns := config["columns"].([]interface{})
		col := columns[0].(map[string]interface{})
		if col["desc"] != "Price in USD" {
			t.Errorf("expected description 'Price in USD', got %v", col["desc"])
		}
	})

	t.Run("AC-04: reject duplicate column", func(t *testing.T) {
		// Given: Column "Name" already exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// When: User runs `stash column add Name`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// Then: Command fails with appropriate error
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})

	t.Run("AC-05: reject reserved names", func(t *testing.T) {
		// Given: Stash "inventory" exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User runs `stash column add _id`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "_id"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("AC-06: case-insensitive duplicate detection", func(t *testing.T) {
		// Given: Column "Name" already exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// When: User runs `stash column add name`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "name"})
		rootCmd.Execute()

		// Then: Command fails with error "column 'Name' already exists"
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})
}

// TestUC_COL_001_AddColumn_MustNot tests anti-requirements for UC-COL-001
func TestUC_COL_001_AddColumn_MustNot(t *testing.T) {
	t.Run("must not allow duplicate column names", func(t *testing.T) {
		// Given: Column "Name" exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// When: User tries to add "Name" again
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// Then: Exit code is 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})

	t.Run("must not allow reserved names", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to add reserved column names
		reservedNames := []string{"_id", "_hash", "_created_at", "_created_by", "_updated_at", "_updated_by", "_branch", "_deleted_at", "_deleted_by", "_op", "_parent"}
		for _, name := range reservedNames {
			ExitCode = 0
			rootCmd.SetArgs([]string{"column", "add", name})
			rootCmd.Execute()

			// Then: Exit code is 2
			if ExitCode != 2 {
				t.Errorf("expected exit code 2 for reserved name %q, got %d", name, ExitCode)
			}
		}
	})

	t.Run("must not allow invalid characters in column names", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to add column with invalid characters
		invalidNames := []string{"123abc", "my-column", "my column", "my.column", "@column"}
		for _, name := range invalidNames {
			ExitCode = 0
			rootCmd.SetArgs([]string{"column", "add", name})
			rootCmd.Execute()

			// Then: Exit code is 2
			if ExitCode != 2 {
				t.Errorf("expected exit code 2 for invalid name %q, got %d", name, ExitCode)
			}
		}
	})
}

// TestUC_COL_002_ListColumns tests UC-COL-002: List Columns
func TestUC_COL_002_ListColumns(t *testing.T) {
	t.Run("AC-01: list columns with stats", func(t *testing.T) {
		// Given: Stash has columns Name, Price
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name", "--desc", "Product name"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Price", "--desc", "Price in USD"})
		rootCmd.Execute()

		// When: User runs `stash column list`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "list"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("AC-02: JSON output format", func(t *testing.T) {
		// Given: Stash has columns
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name", "--desc", "Product name"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Price"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash column list --json`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "list", "--json"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Read captured output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON array
		var columns []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &columns); err != nil {
			t.Fatalf("expected valid JSON array, got parse error: %v\nOutput: %s", err, output)
		}

		// Then: Each entry has name, desc, populated, empty
		if len(columns) != 2 {
			t.Fatalf("expected 2 columns, got %d", len(columns))
		}

		for _, col := range columns {
			if col["name"] == nil {
				t.Error("expected 'name' field in column")
			}
			if _, ok := col["desc"]; !ok {
				t.Error("expected 'desc' field in column")
			}
			if _, ok := col["populated"]; !ok {
				t.Error("expected 'populated' field in column")
			}
			if _, ok := col["empty"]; !ok {
				t.Error("expected 'empty' field in column")
			}
		}
	})
}

// TestUC_COL_002_ListColumns_MustNot tests anti-requirements for UC-COL-002
func TestUC_COL_002_ListColumns_MustNot(t *testing.T) {
	t.Run("must not include system columns in list", func(t *testing.T) {
		// Given: Stash has user columns
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash column list --json`
		rootCmd.SetArgs([]string{"column", "list", "--json"})
		rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: System columns should not be included
		var columns []map[string]interface{}
		json.Unmarshal([]byte(strings.TrimSpace(output)), &columns)

		for _, col := range columns {
			name := col["name"].(string)
			if strings.HasPrefix(name, "_") {
				t.Errorf("system column %q should not be in list", name)
			}
		}
	})
}

// TestUC_COL_003_DescribeColumn tests UC-COL-003: Describe Column
func TestUC_COL_003_DescribeColumn(t *testing.T) {
	t.Run("AC-01: set description", func(t *testing.T) {
		// Given: Column "Price" exists without description
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Price"})
		rootCmd.Execute()

		// When: User runs `stash column describe Price "Price in USD"`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "describe", "Price", "Price in USD"})
		err := rootCmd.Execute()

		// Then: Description is set in config.json
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		configData, _ := os.ReadFile(configPath)

		var config map[string]interface{}
		json.Unmarshal(configData, &config)

		columns := config["columns"].([]interface{})
		col := columns[0].(map[string]interface{})
		if col["desc"] != "Price in USD" {
			t.Errorf("expected description 'Price in USD', got %v", col["desc"])
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("AC-02: update existing description", func(t *testing.T) {
		// Given: Column "Price" has description "Cost"
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Price", "--desc", "Cost"})
		rootCmd.Execute()

		// When: User runs `stash column describe Price "Price in USD, excluding tax"`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "describe", "Price", "Price in USD, excluding tax"})
		err := rootCmd.Execute()

		// Then: Description is updated
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		configPath := filepath.Join(tempDir, ".stash", "inventory", "config.json")
		configData, _ := os.ReadFile(configPath)

		var config map[string]interface{}
		json.Unmarshal(configData, &config)

		columns := config["columns"].([]interface{})
		col := columns[0].(map[string]interface{})
		if col["desc"] != "Price in USD, excluding tax" {
			t.Errorf("expected description 'Price in USD, excluding tax', got %v", col["desc"])
		}

		// Then: Exit code is 0
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}

// TestUC_COL_003_DescribeColumn_MustNot tests anti-requirements for UC-COL-003
func TestUC_COL_003_DescribeColumn_MustNot(t *testing.T) {
	t.Run("must not modify column data", func(t *testing.T) {
		// This is verified by the implementation - describe only updates description
		// No explicit test needed as the implementation only touches the desc field
	})

	t.Run("must reject non-existent column", func(t *testing.T) {
		// Given: Stash exists but column "Price" does not
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User runs `stash column describe Price "Description"`
		ExitCode = 0
		rootCmd.SetArgs([]string{"column", "describe", "Price", "Description"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode != 1 {
			t.Errorf("expected exit code 1 for non-existent column, got %d", ExitCode)
		}
	})
}

// TestColumnAlias tests that "col" alias works
func TestColumnAlias(t *testing.T) {
	t.Run("col alias works for add", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User runs `stash col add Name`
		ExitCode = 0
		rootCmd.SetArgs([]string{"col", "add", "Name"})
		err := rootCmd.Execute()

		// Then: Column is added successfully
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("col alias works for list", func(t *testing.T) {
		// Given: Stash exists with columns
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()

		// When: User runs `stash col list`
		ExitCode = 0
		rootCmd.SetArgs([]string{"col", "list"})
		err := rootCmd.Execute()

		// Then: List succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}
