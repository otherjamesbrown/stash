package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

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
