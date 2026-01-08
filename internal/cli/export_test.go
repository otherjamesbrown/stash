package cli

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetExportFlags resets export command flags
func resetExportFlags() {
	exportFormat = "csv"
	exportOutput = ""
	exportWhere = nil
	exportIncludeDeleted = false
	exportForce = false
}

// TestUC_IMP_002_ExportToFile tests UC-IMP-002: Export to File
func TestUC_IMP_002_ExportToFile(t *testing.T) {
	t.Run("AC-01: export all to CSV", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export products.csv`
		outputFile := filepath.Join(tempDir, "products.csv")
		rootCmd.SetArgs([]string{"export", outputFile})
		err := rootCmd.Execute()

		// Then: CSV file is created with headers
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: All active records are exported
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		reader := csv.NewReader(strings.NewReader(string(content)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}

		// Should have header + 2 data rows
		if len(records) != 3 {
			t.Errorf("expected 3 rows (header + 2 records), got %d", len(records))
		}

		// Check header
		if records[0][0] != "Name" || records[0][1] != "Price" {
			t.Errorf("expected header [Name, Price], got %v", records[0])
		}
	})

	t.Run("AC-02: export to JSON", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export products.json --format json`
		outputFile := filepath.Join(tempDir, "products.json")
		rootCmd.SetArgs([]string{"export", outputFile, "--format", "json"})
		err := rootCmd.Execute()

		// Then: JSON file is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Contains array of records
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		var records []map[string]interface{}
		if err := json.Unmarshal(content, &records); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("AC-03: filtered export", func(t *testing.T) {
		// Given: Stash has records with various Categories
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export electronics.csv --where "Category=electronics"`
		outputFile := filepath.Join(tempDir, "electronics.csv")
		rootCmd.SetArgs([]string{"export", outputFile, "--where", "Category=electronics"})
		err := rootCmd.Execute()

		// Then: Only matching records are exported
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		reader := csv.NewReader(strings.NewReader(string(content)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}

		// Should have header + 2 electronics records
		if len(records) != 3 {
			t.Errorf("expected 3 rows (header + 2 electronics), got %d", len(records))
		}

		// Verify only electronics are exported
		for i := 1; i < len(records); i++ {
			if records[i][1] != "electronics" {
				t.Errorf("expected Category=electronics, got %s", records[i][1])
			}
		}
	})

	t.Run("AC-04: include deleted records", func(t *testing.T) {
		// Given: Stash has active and deleted records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "ToDelete"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Delete one record using rm command
		// First get the record ID
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		rootCmd.SetArgs([]string{"list", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		var listRecords []map[string]interface{}
		json.Unmarshal([]byte(output), &listRecords)

		var deleteID string
		for _, rec := range listRecords {
			if rec["Name"] == "ToDelete" {
				deleteID = rec["_id"].(string)
				break
			}
		}

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"rm", deleteID, "--yes"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export all.csv --include-deleted`
		outputFile := filepath.Join(tempDir, "all.csv")
		rootCmd.SetArgs([]string{"export", outputFile, "--include-deleted"})
		err := rootCmd.Execute()

		// Then: Both active and deleted records are exported
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		reader := csv.NewReader(strings.NewReader(string(content)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}

		// Should have header + 2 records (including deleted)
		if len(records) != 3 {
			t.Errorf("expected 3 rows (header + 2 records including deleted), got %d", len(records))
		}
	})

	t.Run("export to stdout when no file specified", func(t *testing.T) {
		// Given: Stash has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash export` (no file)
		rootCmd.SetArgs([]string{"export"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is CSV to stdout
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should contain CSV header and data
		if !strings.Contains(output, "Name") {
			t.Error("expected CSV header in stdout")
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop in stdout")
		}
	})

	t.Run("export JSONL format", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export products.jsonl --format jsonl`
		outputFile := filepath.Join(tempDir, "products.jsonl")
		rootCmd.SetArgs([]string{"export", outputFile, "--format", "jsonl"})
		err := rootCmd.Execute()

		// Then: JSONL file is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Each line should be valid JSON
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}

		for i, line := range lines {
			var rec map[string]interface{}
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Errorf("line %d is not valid JSON: %v", i+1, err)
			}
		}
	})

	t.Run("refuse to overwrite existing file without --force", func(t *testing.T) {
		// Given: Output file already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// Create existing file
		outputFile := filepath.Join(tempDir, "existing.csv")
		os.WriteFile(outputFile, []byte("existing content"), 0644)

		// When: User runs `stash export existing.csv` (without --force)
		rootCmd.SetArgs([]string{"export", outputFile})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when file exists")
		}

		// File should not be modified
		content, _ := os.ReadFile(outputFile)
		if string(content) != "existing content" {
			t.Error("expected file to not be modified")
		}
	})

	t.Run("overwrite existing file with --force", func(t *testing.T) {
		// Given: Output file already exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// Create existing file
		outputFile := filepath.Join(tempDir, "existing.csv")
		os.WriteFile(outputFile, []byte("existing content"), 0644)

		// When: User runs `stash export existing.csv --force`
		rootCmd.SetArgs([]string{"export", outputFile, "--force"})
		err := rootCmd.Execute()

		// Then: File is overwritten
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		content, _ := os.ReadFile(outputFile)
		if string(content) == "existing content" {
			t.Error("expected file to be overwritten")
		}
		if !strings.Contains(string(content), "Name") {
			t.Error("expected CSV content")
		}
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()
		resetExportFlags()

		// When: User runs `stash export --format invalid`
		rootCmd.SetArgs([]string{"export", "--format", "invalid"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for invalid format")
		}
	})

	t.Run("no stash returns error", func(t *testing.T) {
		// Given: No stash directory
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetExportFlags()

		// When: User runs `stash export`
		rootCmd.SetArgs([]string{"export"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})
}
