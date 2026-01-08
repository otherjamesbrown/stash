package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// resetImportFlags resets import command flags
func resetImportFlags() {
	importConfirm = false
	importDryRun = false
	importColumn = ""
	importFormat = ""
}

// TestUC_IMP_001_ImportFromCSV tests UC-IMP-001: Import from CSV
func TestUC_IMP_001_ImportFromCSV(t *testing.T) {
	t.Run("AC-02: skip confirmation with --confirm", func(t *testing.T) {
		// Given: CSV file with Name, Category, Price columns exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV file
		csvContent := "Name,Category,Price\nLaptop,electronics,999\nMouse,accessories,50\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.csv --confirm`
		rootCmd.SetArgs([]string{"import", csvFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Import proceeds without prompting
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify records were created
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("AC-03: dry run preview", func(t *testing.T) {
		// Given: CSV file exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV file
		csvContent := "Name,Category,Price\nLaptop,electronics,999\nMouse,accessories,50\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash import products.csv --dry-run`
		rootCmd.SetArgs([]string{"import", csvFile, "--dry-run"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows what would be imported
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: No records are created
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 0 {
			t.Errorf("expected 0 records (dry run), got %d", len(records))
		}

		// Output should mention dry run
		if !strings.Contains(output, "Dry run") && !strings.Contains(output, "dry run") {
			t.Errorf("expected 'dry run' in output, got: %s", output)
		}
	})

	t.Run("AC-04: specify primary column", func(t *testing.T) {
		// Given: CSV has columns ProductName, Price
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV file with ProductName as first column
		csvContent := "ProductName,Price\nLaptop,999\nMouse,50\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.csv --column ProductName --confirm`
		rootCmd.SetArgs([]string{"import", csvFile, "--column", "ProductName", "--confirm"})
		err := rootCmd.Execute()

		// Then: ProductName is used as primary column
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify records were created with ProductName field
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}

		// Check that ProductName field exists
		found := false
		for _, rec := range records {
			if rec.Fields["ProductName"] == "Laptop" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find record with ProductName=Laptop")
		}
	})

	t.Run("AC-05: handle missing values", func(t *testing.T) {
		// Given: CSV has rows with empty cells
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV with empty cells
		csvContent := "Name,Category,Price\nLaptop,,999\nMouse,accessories,\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.csv --confirm`
		rootCmd.SetArgs([]string{"import", csvFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Empty cells are imported as empty strings
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Records are created successfully
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}

		// Check that empty values are handled
		for _, rec := range records {
			if rec.Fields["Name"] == "Laptop" {
				// Category should be empty string
				if rec.Fields["Category"] != "" {
					t.Errorf("expected empty Category for Laptop, got %v", rec.Fields["Category"])
				}
			}
		}
	})

	t.Run("creates missing columns automatically", func(t *testing.T) {
		// Given: Stash has only Name column
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV with additional columns
		csvContent := "Name,Category,Price\nLaptop,electronics,999\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.csv --confirm`
		rootCmd.SetArgs([]string{"import", csvFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Missing columns are created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify columns exist
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("inventory")
		if !stash.Columns.Exists("Category") {
			t.Error("expected Category column to be created")
		}
		if !stash.Columns.Exists("Price") {
			t.Error("expected Price column to be created")
		}
	})

	t.Run("import JSON array format", func(t *testing.T) {
		// Given: JSON file with array of objects
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create JSON file
		jsonContent := `[{"Name": "Laptop", "Price": 999}, {"Name": "Mouse", "Price": 50}]`
		jsonFile := filepath.Join(tempDir, "products.json")
		os.WriteFile(jsonFile, []byte(jsonContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.json --confirm`
		rootCmd.SetArgs([]string{"import", jsonFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Records are imported
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("import JSONL format", func(t *testing.T) {
		// Given: JSONL file with one object per line
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create JSONL file
		jsonlContent := `{"Name": "Laptop", "Price": 999}
{"Name": "Mouse", "Price": 50}`
		jsonlFile := filepath.Join(tempDir, "products.jsonl")
		os.WriteFile(jsonlFile, []byte(jsonlContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.jsonl --confirm`
		rootCmd.SetArgs([]string{"import", jsonlFile, "--confirm"})
		err := rootCmd.Execute()

		// Then: Records are imported
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("file not found returns error", func(t *testing.T) {
		// Given: File does not exist
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()
		resetImportFlags()

		// When: User runs `stash import nonexistent.csv --confirm`
		rootCmd.SetArgs([]string{"import", "nonexistent.csv", "--confirm"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for missing file")
		}
	})

	t.Run("no stash returns error", func(t *testing.T) {
		// Given: No stash directory
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetImportFlags()

		// Create a CSV file
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte("Name\nLaptop"), 0644)

		// When: User runs `stash import`
		rootCmd.SetArgs([]string{"import", csvFile, "--confirm"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})

	t.Run("JSON output for dry run", func(t *testing.T) {
		// Given: CSV file exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV file
		csvContent := "Name,Category\nLaptop,electronics\n"
		csvFile := filepath.Join(tempDir, "products.csv")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash import products.csv --dry-run --json`
		rootCmd.SetArgs([]string{"import", csvFile, "--dry-run", "--json"})
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

		// Check dry_run flag
		if result["dry_run"] != true {
			t.Error("expected dry_run=true in JSON output")
		}
	})

	t.Run("explicit format flag overrides extension", func(t *testing.T) {
		// Given: CSV file with .txt extension
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create CSV file with wrong extension
		csvContent := "Name,Price\nLaptop,999\n"
		csvFile := filepath.Join(tempDir, "products.txt")
		os.WriteFile(csvFile, []byte(csvContent), 0644)

		resetImportFlags()

		// When: User runs `stash import products.txt --format csv --confirm`
		rootCmd.SetArgs([]string{"import", csvFile, "--format", "csv", "--confirm"})
		err := rootCmd.Execute()

		// Then: File is parsed as CSV
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		if len(records) != 1 {
			t.Errorf("expected 1 record, got %d", len(records))
		}
	})

	t.Run("invalid JSON format returns error", func(t *testing.T) {
		// Given: Invalid JSON file
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create invalid JSON file
		jsonFile := filepath.Join(tempDir, "invalid.json")
		os.WriteFile(jsonFile, []byte("{invalid json}"), 0644)

		resetImportFlags()

		// When: User runs `stash import invalid.json --confirm`
		rootCmd.SetArgs([]string{"import", jsonFile, "--confirm"})
		rootCmd.Execute()

		// Then: Command fails with error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for invalid JSON")
		}
	})
}
