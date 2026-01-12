package cli

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// resetQueryFlags resets query command flags
func resetQueryFlags() {
	queryCSV = false
	queryNoHeaders = false
	queryColumns = ""
}

// TestUC_QRY_003_RawSQLQuery tests UC-QRY-003: Raw SQL Query
func TestUC_QRY_003_RawSQLQuery(t *testing.T) {
	t.Run("AC-01: execute SELECT query", func(t *testing.T) {
		// Given: Stash "inventory" has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create some records
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=25"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Keyboard", "--set", "Price=150"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Price FROM inventory WHERE Price > 100"`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Price FROM inventory WHERE CAST(Price AS INTEGER) > 100"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Query results are displayed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should contain Laptop and Keyboard (Price > 100)
		if !strings.Contains(output, "Laptop") {
			t.Error("expected output to contain 'Laptop'")
		}
		if !strings.Contains(output, "Keyboard") {
			t.Error("expected output to contain 'Keyboard'")
		}
		// Should not contain Mouse (Price = 25)
		if strings.Contains(output, "Mouse") {
			t.Error("expected output NOT to contain 'Mouse'")
		}
	})

	t.Run("AC-02: reject non-SELECT queries", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Add a record
		rootCmd.SetArgs([]string{"add", "Test"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// When: User runs `stash query "DELETE FROM inventory"`
		rootCmd.SetArgs([]string{"query", "DELETE FROM inventory"})
		rootCmd.Execute()

		// Then: Command fails with appropriate error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for DELETE query")
		}

		// Verify: No data is modified
		store, _ := storage.NewStore(filepath.Join(t.TempDir(), ".stash"))
		// Note: We can't easily verify the original temp dir here, but the test
		// ensures the command fails before any modification
		store.Close()
	})

	t.Run("AC-02: reject INSERT queries", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash query "INSERT INTO inventory ..."`
		rootCmd.SetArgs([]string{"query", "INSERT INTO inventory (id) VALUES ('test')"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for INSERT query")
		}
	})

	t.Run("AC-02: reject UPDATE queries", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash query "UPDATE inventory SET ..."`
		rootCmd.SetArgs([]string{"query", "UPDATE inventory SET Name='Hacked'"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for UPDATE query")
		}
	})

	t.Run("AC-02: reject DROP queries", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash query "DROP TABLE inventory"`
		rootCmd.SetArgs([]string{"query", "DROP TABLE inventory"})
		rootCmd.Execute()

		// Then: Command fails
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for DROP query")
		}
	})

	t.Run("AC-03: JSON output", func(t *testing.T) {
		// Given: Stash has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create records
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=25"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT * FROM inventory" --json`
		rootCmd.SetArgs([]string{"query", "SELECT * FROM inventory", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 16384)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON array
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		var jsonResult []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonResult); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		if len(jsonResult) != 2 {
			t.Errorf("expected 2 records, got %d", len(jsonResult))
		}
	})

	t.Run("AC-04: aggregation queries", func(t *testing.T) {
		// Given: Stash has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		// Create records with different categories
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Category=electronics"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Category, COUNT(*) FROM inventory GROUP BY Category"`
		rootCmd.SetArgs([]string{"query", "SELECT Category, COUNT(*) as count FROM inventory GROUP BY Category"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Aggregation results are displayed
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Should show category counts
		if !strings.Contains(output, "electronics") {
			t.Error("expected output to contain 'electronics'")
		}
		if !strings.Contains(output, "furniture") {
			t.Error("expected output to contain 'furniture'")
		}
	})
}

// TestUC_QRY_003_RawSQLQuery_MustNot tests anti-requirements
func TestUC_QRY_003_RawSQLQuery_MustNot(t *testing.T) {
	t.Run("must not allow data modification via SQL", func(t *testing.T) {
		// Given: Stash has records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Add a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		resetFlags()
		ExitCode = 0

		// Get initial record count
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		initialCount, _ := store.CountRecords("inventory")
		store.Close()

		// When: User tries to modify data via SQL
		modifyQueries := []string{
			"DELETE FROM inventory",
			"UPDATE inventory SET Name='Hacked'",
			"INSERT INTO inventory (id, Name) VALUES ('test', 'Test')",
			"DROP TABLE inventory",
		}

		for _, query := range modifyQueries {
			ExitCode = 0
			resetFlags()
			rootCmd.SetArgs([]string{"query", query})
			rootCmd.Execute()

			// Then: Command should fail
			if ExitCode == 0 {
				t.Errorf("expected non-zero exit code for query: %s", query)
			}
		}

		// Verify: Data is unchanged
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()
		finalCount, _ := store.CountRecords("inventory")
		if finalCount != initialCount {
			t.Errorf("data was modified: expected %d records, got %d", initialCount, finalCount)
		}
	})
}

// TestQueryCSVOutput tests CSV output functionality for query command
func TestQueryCSVOutput(t *testing.T) {
	t.Run("CSV output with --csv flag", func(t *testing.T) {
		// Given: Stash has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create records
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		resetFlags()
		resetQueryFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=25"})
		rootCmd.Execute()
		resetFlags()
		resetQueryFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Price FROM inventory" --csv`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Price FROM inventory", "--csv"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid CSV
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Parse as CSV
		reader := csv.NewReader(strings.NewReader(output))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("expected valid CSV, got error: %v\nOutput: %s", err, output)
		}

		// Should have header + 2 data rows
		if len(records) != 3 {
			t.Errorf("expected 3 rows (header + 2 records), got %d", len(records))
		}

		// Check header
		if records[0][0] != "Name" || records[0][1] != "Price" {
			t.Errorf("expected header [Name, Price], got %v", records[0])
		}

		// Data should be present
		foundLaptop := false
		foundMouse := false
		for i := 1; i < len(records); i++ {
			if records[i][0] == "Laptop" {
				foundLaptop = true
			}
			if records[i][0] == "Mouse" {
				foundMouse = true
			}
		}
		if !foundLaptop {
			t.Error("expected to find Laptop in CSV output")
		}
		if !foundMouse {
			t.Error("expected to find Mouse in CSV output")
		}
	})

	t.Run("CSV output with --no-headers flag", func(t *testing.T) {
		// Given: Stash has records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		resetFlags()
		resetQueryFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Price FROM inventory" --csv --no-headers`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Price FROM inventory", "--csv", "--no-headers"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is CSV without header
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Parse as CSV
		reader := csv.NewReader(strings.NewReader(output))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("expected valid CSV, got error: %v\nOutput: %s", err, output)
		}

		// Should have only 1 data row (no header)
		if len(records) != 1 {
			t.Errorf("expected 1 row (no header), got %d", len(records))
		}

		// First row should be data, not header
		if records[0][0] != "Laptop" {
			t.Errorf("expected first row to be data 'Laptop', got %v", records[0][0])
		}
	})

	t.Run("CSV output with --columns flag for column selection", func(t *testing.T) {
		// Given: Stash has records with multiple fields
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999", "--set", "Category=electronics"})
		rootCmd.Execute()
		resetFlags()
		resetQueryFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT * FROM inventory" --csv --columns "Name,Price"`
		rootCmd.SetArgs([]string{"query", "SELECT * FROM inventory", "--csv", "--columns", "Name,Price"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output contains only selected columns
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Parse as CSV
		reader := csv.NewReader(strings.NewReader(output))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("expected valid CSV, got error: %v\nOutput: %s", err, output)
		}

		// Should have header + 1 data row
		if len(records) != 2 {
			t.Errorf("expected 2 rows (header + 1 record), got %d", len(records))
		}

		// Check header has only selected columns
		if len(records[0]) != 2 {
			t.Errorf("expected 2 columns, got %d", len(records[0]))
		}
		if records[0][0] != "Name" || records[0][1] != "Price" {
			t.Errorf("expected header [Name, Price], got %v", records[0])
		}
	})

	t.Run("CSV properly escapes special characters", func(t *testing.T) {
		// Given: Stash has records with special characters
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Description"})
		defer cleanup()

		// Create a record with commas and quotes in the value
		rootCmd.SetArgs([]string{"add", "Item, with comma", "--set", `Description=Contains "quotes" and, commas`})
		rootCmd.Execute()
		resetFlags()
		resetQueryFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Description FROM inventory" --csv`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Description FROM inventory", "--csv"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid CSV with proper escaping
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Parse as CSV - this will fail if escaping is incorrect
		reader := csv.NewReader(strings.NewReader(output))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("CSV parsing failed (bad escaping): %v\nOutput: %s", err, output)
		}

		// Should have header + 1 data row
		if len(records) != 2 {
			t.Errorf("expected 2 rows, got %d", len(records))
		}

		// Data row should have correctly parsed values
		if records[1][0] != "Item, with comma" {
			t.Errorf("expected Name to be 'Item, with comma', got '%s'", records[1][0])
		}
		if records[1][1] != `Contains "quotes" and, commas` {
			t.Errorf("expected Description with special chars, got '%s'", records[1][1])
		}
	})

	t.Run("CSV output with empty result set", func(t *testing.T) {
		// Given: Stash has no records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Price FROM inventory" --csv`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Price FROM inventory", "--csv"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is CSV with only header
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Parse as CSV
		reader := csv.NewReader(strings.NewReader(output))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("expected valid CSV, got error: %v\nOutput: %s", err, output)
		}

		// Should have only header row
		if len(records) != 1 {
			t.Errorf("expected 1 row (header only), got %d", len(records))
		}

		// Check header
		if records[0][0] != "Name" || records[0][1] != "Price" {
			t.Errorf("expected header [Name, Price], got %v", records[0])
		}
	})

	t.Run("CSV with --no-headers and empty result gives no output", func(t *testing.T) {
		// Given: Stash has no records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()
		resetQueryFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash query "SELECT Name, Price FROM inventory" --csv --no-headers`
		rootCmd.SetArgs([]string{"query", "SELECT Name, Price FROM inventory", "--csv", "--no-headers"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output should be empty
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Output should be empty (no header, no data)
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output, got '%s'", output)
		}
	})
}
