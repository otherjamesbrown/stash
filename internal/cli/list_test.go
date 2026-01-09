package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_004_ListRecords tests listing records (referenced in records.yaml as part of UC-REC-005 AC-01)
func TestUC_REC_004_ListRecords(t *testing.T) {
	t.Run("list root records by default", func(t *testing.T) {
		// Given: Stash with multiple records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create multiple records
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Keyboard"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list`
		rootCmd.SetArgs([]string{"list"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: All records are shown
		if !strings.Contains(output, "Laptop") {
			t.Error("expected output to contain Laptop")
		}
		if !strings.Contains(output, "Mouse") {
			t.Error("expected output to contain Mouse")
		}
		if !strings.Contains(output, "Keyboard") {
			t.Error("expected output to contain Keyboard")
		}

		// Then: Shows count
		if !strings.Contains(output, "3 record") {
			t.Error("expected output to show record count")
		}
	})

	t.Run("list with JSON output", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --json`
		rootCmd.SetArgs([]string{"list", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON array
		var jsonOutput []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		// Then: Contains all records
		if len(jsonOutput) != 2 {
			t.Errorf("expected 2 records, got %d", len(jsonOutput))
		}
	})

	t.Run("list excludes deleted records by default", func(t *testing.T) {
		// Given: Stash with deleted records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "DeleteMe"})
		rootCmd.Execute()

		// Delete the second record
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		var deleteID string
		for _, rec := range records {
			if rec.Fields["Name"] == "DeleteMe" {
				deleteID = rec.ID
				break
			}
		}
		store.DeleteRecord("inventory", deleteID, "test")
		store.Close()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list`
		rootCmd.SetArgs([]string{"list"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Deleted record is not shown
		if strings.Contains(output, "DeleteMe") {
			t.Error("expected deleted record to be excluded from list")
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected active record to be shown")
		}
	})

	t.Run("list includes deleted records with --deleted", func(t *testing.T) {
		// Given: Stash with deleted records
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "DeleteMe"})
		rootCmd.Execute()

		// Delete the second record
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		var deleteID string
		for _, rec := range records {
			if rec.Fields["Name"] == "DeleteMe" {
				deleteID = rec.ID
				break
			}
		}
		store.DeleteRecord("inventory", deleteID, "test")
		store.Close()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --deleted`
		rootCmd.SetArgs([]string{"list", "--deleted"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Both records are shown
		if !strings.Contains(output, "DeleteMe") {
			t.Error("expected deleted record to be shown with --deleted")
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected active record to be shown")
		}
	})

	t.Run("list children with --parent", func(t *testing.T) {
		// Given: Record with children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create children
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Battery", "--parent", parentID})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --parent inv-xxxx`
		rootCmd.SetArgs([]string{"list", "--parent", parentID})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only children are shown
		if strings.Contains(output, "Laptop") {
			t.Error("expected parent not to be shown with --parent filter")
		}
		if !strings.Contains(output, "Charger") {
			t.Error("expected child 'Charger' to be shown")
		}
		if !strings.Contains(output, "Battery") {
			t.Error("expected child 'Battery' to be shown")
		}
	})

	t.Run("list all records with --all", func(t *testing.T) {
		// Given: Records with children
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create parent
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		parentID := records[0].ID
		store.Close()

		// Create child
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Charger", "--parent", parentID})
		rootCmd.Execute()
		ExitCode = 0

		// Create another root
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --all`
		rootCmd.SetArgs([]string{"list", "--all"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: All records including children are shown
		if !strings.Contains(output, "Laptop") {
			t.Error("expected parent to be shown")
		}
		if !strings.Contains(output, "Charger") {
			t.Error("expected child to be shown with --all")
		}
		if !strings.Contains(output, "Mouse") {
			t.Error("expected other root to be shown")
		}
		if !strings.Contains(output, "3 record") {
			t.Error("expected 3 records total")
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		// Given: Many records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		for i := 0; i < 5; i++ {
			ExitCode = 0
			rootCmd.SetArgs([]string{"add", "Item"})
			rootCmd.Execute()
		}
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --limit 2`
		rootCmd.SetArgs([]string{"list", "--limit", "2"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only 2 records are shown
		if !strings.Contains(output, "2 record") {
			t.Errorf("expected 2 records with limit, output: %s", output)
		}
	})

	t.Run("list with no records", func(t *testing.T) {
		// Given: Empty stash
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list`
		rootCmd.SetArgs([]string{"list"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Shows "No records found"
		if !strings.Contains(output, "No records found") {
			t.Error("expected 'No records found' message")
		}
	})

	t.Run("list with no stash", func(t *testing.T) {
		// Given: No stash directory
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash list`
		rootCmd.SetArgs([]string{"list"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})
}

// TestUC_QRY_001_FilterRecords tests filtering functionality per UC-QRY-001
func TestUC_QRY_001_FilterRecords(t *testing.T) {
	t.Run("AC-02: filter with WHERE clause equality", func(t *testing.T) {
		// Given: Stash has records with various Categories
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Category=electronics"`
		rootCmd.SetArgs([]string{"list", "--where", "Category=electronics"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only matching records are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (electronics) to be shown")
		}
		if !strings.Contains(output, "Phone") {
			t.Error("expected Phone (electronics) to be shown")
		}
		if strings.Contains(output, "Desk") {
			t.Error("expected Desk (furniture) to NOT be shown")
		}
		if !strings.Contains(output, "2 record") {
			t.Errorf("expected 2 records, output: %s", output)
		}
	})

	t.Run("AC-02: filter with WHERE clause not-equals", func(t *testing.T) {
		// Given: Stash has records with various Categories
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Phone", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Category!=electronics"`
		rootCmd.SetArgs([]string{"list", "--where", "Category!=electronics"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only non-matching records are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (electronics) to NOT be shown")
		}
		if strings.Contains(output, "Phone") {
			t.Error("expected Phone (electronics) to NOT be shown")
		}
		if !strings.Contains(output, "Desk") {
			t.Error("expected Desk (furniture) to be shown")
		}
	})

	t.Run("AC-02: filter with WHERE clause LIKE", func(t *testing.T) {
		// Given: Stash has records with various Names
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop Pro", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Standing Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Laptop Air", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Name LIKE Laptop%"`
		rootCmd.SetArgs([]string{"list", "--where", "Name LIKE Laptop%"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only matching records are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop Pro") {
			t.Error("expected 'Laptop Pro' to be shown")
		}
		if !strings.Contains(output, "Laptop Air") {
			t.Error("expected 'Laptop Air' to be shown")
		}
		if strings.Contains(output, "Standing Desk") {
			t.Error("expected 'Standing Desk' to NOT be shown")
		}
	})

	t.Run("AC-02: filter with WHERE clause numeric comparison", func(t *testing.T) {
		// Given: Stash has records with various Prices
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Monitor", "--set", "Price=300"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Price>100"`
		rootCmd.SetArgs([]string{"list", "--where", "Price>100"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records with Price > 100 are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (999) to be shown")
		}
		if !strings.Contains(output, "Monitor") {
			t.Error("expected Monitor (300) to be shown")
		}
		if strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (50) to NOT be shown")
		}
	})

	t.Run("multiple WHERE conditions with AND logic", func(t *testing.T) {
		// Given: Stash has records with various Categories and Prices
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics", "--set", "Price=999"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Category=electronics", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture", "--set", "Price=300"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Category=electronics" --where "Price>100"`
		rootCmd.SetArgs([]string{"list", "--where", "Category=electronics", "--where", "Price>100"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records matching ALL conditions are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (electronics, 999) to be shown")
		}
		if strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (electronics, 50) to NOT be shown")
		}
		if strings.Contains(output, "Desk") {
			t.Error("expected Desk (furniture, 300) to NOT be shown")
		}
	})

	t.Run("search across all fields", func(t *testing.T) {
		// Given: Stash has records with searchable content
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Description=Powerful computing device"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Description=Input device for navigation"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Keyboard Pro", "--set", "Description=Mechanical keyboard"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --search "device"`
		rootCmd.SetArgs([]string{"list", "--search", "device"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records containing "device" in any field are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (computing device) to be shown")
		}
		if !strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (Input device) to be shown")
		}
		if strings.Contains(output, "Keyboard Pro") {
			t.Error("expected Keyboard Pro to NOT be shown (no 'device' match)")
		}
	})

	t.Run("order-by with ascending sort", func(t *testing.T) {
		// Given: Stash has records with various Names
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Zebra", "--set", "Price=100"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Price=200"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Banana", "--set", "Price=50"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --order-by Name`
		rootCmd.SetArgs([]string{"list", "--order-by", "Name"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records are sorted by Name ascending
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		appleIdx := strings.Index(output, "Apple")
		bananaIdx := strings.Index(output, "Banana")
		zebraIdx := strings.Index(output, "Zebra")

		if appleIdx == -1 || bananaIdx == -1 || zebraIdx == -1 {
			t.Fatalf("expected all records to be shown, output: %s", output)
		}
		if !(appleIdx < bananaIdx && bananaIdx < zebraIdx) {
			t.Errorf("expected Apple < Banana < Zebra order, output: %s", output)
		}
	})

	t.Run("order-by with descending sort", func(t *testing.T) {
		// Given: Stash has records with various Prices
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Cheap", "--set", "Price=10"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Medium", "--set", "Price=100"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Expensive", "--set", "Price=1000"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --order-by Price --desc`
		rootCmd.SetArgs([]string{"list", "--order-by", "Price", "--desc"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records are sorted by Price descending
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expIdx := strings.Index(output, "Expensive")
		medIdx := strings.Index(output, "Medium")
		cheapIdx := strings.Index(output, "Cheap")

		if expIdx == -1 || medIdx == -1 || cheapIdx == -1 {
			t.Fatalf("expected all records to be shown, output: %s", output)
		}
		if !(expIdx < medIdx && medIdx < cheapIdx) {
			t.Errorf("expected Expensive < Medium < Cheap order (descending), output: %s", output)
		}
	})

	t.Run("AC-07: select specific columns", func(t *testing.T) {
		// Given: Stash has columns Name, Price, Category
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Price=999", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --columns "Name,Price"`
		rootCmd.SetArgs([]string{"list", "--columns", "Name,Price"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output shows only specified columns
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Name") {
			t.Error("expected Name column header")
		}
		if !strings.Contains(output, "Price") || !strings.Contains(output, "999") {
			t.Error("expected Price column with value")
		}
		// Category should not appear in output (unless as part of column values which it's not)
		// This is a bit tricky to test since we're checking headers, not data
	})

	t.Run("filter with WHERE and JSON output", func(t *testing.T) {
		// Given: Stash has records with various Categories
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Desk", "--set", "Category=furniture"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Category=electronics" --json`
		rootCmd.SetArgs([]string{"list", "--where", "Category=electronics", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON with filtered records
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var records []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &records); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if len(records) != 1 {
			t.Errorf("expected 1 record (electronics only), got %d", len(records))
		}
		if len(records) > 0 {
			if records[0]["Name"] != "Laptop" {
				t.Errorf("expected Laptop, got %v", records[0]["Name"])
			}
		}
	})

	t.Run("invalid WHERE clause format", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0

		// When: User runs `stash list --where "invalid"`
		rootCmd.SetArgs([]string{"list", "--where", "invalid"})
		rootCmd.Execute()

		// Then: Command fails with error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for invalid WHERE clause")
		}
	})

	t.Run("WHERE clause with quoted values", func(t *testing.T) {
		// Given: Stash has records with spaces in values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop Pro", "--set", "Category=office equipment"})
		rootCmd.Execute()
		ExitCode = 0
		rootCmd.SetArgs([]string{"add", "Mouse", "--set", "Category=accessories"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Category='office equipment'"`
		rootCmd.SetArgs([]string{"list", "--where", "Category='office equipment'"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records with quoted value are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop Pro") {
			t.Errorf("expected 'Laptop Pro' to be shown, output: %s", output)
		}
		if strings.Contains(output, "Mouse") {
			t.Error("expected Mouse to NOT be shown")
		}
	})

	t.Run("case insensitive field matching in WHERE", func(t *testing.T) {
		// Given: Stash has column "Category" (title case)
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Category=electronics"})
		rootCmd.Execute()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "category=electronics"` (lowercase)
		rootCmd.SetArgs([]string{"list", "--where", "category=electronics"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Matching is case-insensitive for field name
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Errorf("expected Laptop to be shown (case-insensitive match), output: %s", output)
		}
	})

	t.Run("AC-02: filter with WHERE clause IS NULL", func(t *testing.T) {
		// Given: Stash has records where some have NULL values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		// Create record with Notes set
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record without Notes (NULL)
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Notes IS NULL"`
		rootCmd.SetArgs([]string{"list", "--where", "Notes IS NULL"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records with NULL Notes are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "Mouse") {
			t.Errorf("expected Mouse (NULL Notes) to be shown, output: %s", output)
		}
		if strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (has Notes) to NOT be shown")
		}
	})

	t.Run("AC-02: filter with WHERE clause IS NOT NULL", func(t *testing.T) {
		// Given: Stash has records where some have NULL values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		// Create record with Notes set
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record without Notes (NULL)
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Notes IS NOT NULL"`
		rootCmd.SetArgs([]string{"list", "--where", "Notes IS NOT NULL"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records with non-NULL Notes are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (has Notes) to be shown")
		}
		if strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (NULL Notes) to NOT be shown")
		}
	})

	t.Run("AC-02: filter with WHERE clause IS EMPTY", func(t *testing.T) {
		// Given: Stash has records with NULL, empty string, and populated values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		// Create record with Notes set
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record without Notes (NULL)
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record with empty Notes
		rootCmd.SetArgs([]string{"add", "Keyboard", "--set", "Notes="})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Notes IS EMPTY"`
		rootCmd.SetArgs([]string{"list", "--where", "Notes IS EMPTY"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records with NULL or empty Notes are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (NULL Notes) to be shown")
		}
		if !strings.Contains(output, "Keyboard") {
			t.Error("expected Keyboard (empty Notes) to be shown")
		}
		if strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (has Notes) to NOT be shown")
		}
	})

	t.Run("AC-02: filter with WHERE clause IS NOT EMPTY", func(t *testing.T) {
		// Given: Stash has records with NULL, empty string, and populated values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		// Create record with Notes set
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record without Notes (NULL)
		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Create record with empty Notes
		rootCmd.SetArgs([]string{"add", "Keyboard", "--set", "Notes="})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --where "Notes IS NOT EMPTY"`
		rootCmd.SetArgs([]string{"list", "--where", "Notes IS NOT EMPTY"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records with non-empty Notes are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Laptop") {
			t.Error("expected Laptop (has Notes) to be shown")
		}
		if strings.Contains(output, "Mouse") {
			t.Error("expected Mouse (NULL Notes) to NOT be shown")
		}
		if strings.Contains(output, "Keyboard") {
			t.Error("expected Keyboard (empty Notes) to NOT be shown")
		}
	})
}

// TestParseWhereClause tests the WHERE clause parser
func TestParseWhereClause(t *testing.T) {
	tests := []struct {
		input    string
		expected storage.WhereCondition
		wantErr  bool
	}{
		// Existing operators
		{"field=value", storage.WhereCondition{Field: "field", Operator: "=", Value: "value"}, false},
		{"field!=value", storage.WhereCondition{Field: "field", Operator: "!=", Value: "value"}, false},
		{"field>100", storage.WhereCondition{Field: "field", Operator: ">", Value: "100"}, false},
		{"field<100", storage.WhereCondition{Field: "field", Operator: "<", Value: "100"}, false},
		{"field>=100", storage.WhereCondition{Field: "field", Operator: ">=", Value: "100"}, false},
		{"field<=100", storage.WhereCondition{Field: "field", Operator: "<=", Value: "100"}, false},
		{"field LIKE %pattern%", storage.WhereCondition{Field: "field", Operator: "LIKE", Value: "%pattern%"}, false},

		// IS NULL / IS NOT NULL
		{"field IS NULL", storage.WhereCondition{Field: "field", Operator: "IS NULL", Value: ""}, false},
		{"field IS NOT NULL", storage.WhereCondition{Field: "field", Operator: "IS NOT NULL", Value: ""}, false},

		// IS EMPTY / IS NOT EMPTY
		{"field IS EMPTY", storage.WhereCondition{Field: "field", Operator: "IS EMPTY", Value: ""}, false},
		{"field IS NOT EMPTY", storage.WhereCondition{Field: "field", Operator: "IS NOT EMPTY", Value: ""}, false},

		// Case insensitivity
		{"field is null", storage.WhereCondition{Field: "field", Operator: "IS NULL", Value: ""}, false},
		{"field is not null", storage.WhereCondition{Field: "field", Operator: "IS NOT NULL", Value: ""}, false},
		{"field is empty", storage.WhereCondition{Field: "field", Operator: "IS EMPTY", Value: ""}, false},
		{"field is not empty", storage.WhereCondition{Field: "field", Operator: "IS NOT EMPTY", Value: ""}, false},
		{"Field Is Empty", storage.WhereCondition{Field: "Field", Operator: "IS EMPTY", Value: ""}, false},
		{"Field IS Not NULL", storage.WhereCondition{Field: "Field", Operator: "IS NOT NULL", Value: ""}, false},

		// Whitespace handling
		{"  field IS NULL  ", storage.WhereCondition{Field: "field", Operator: "IS NULL", Value: ""}, false},
		{"field   IS   NOT   NULL", storage.WhereCondition{Field: "field", Operator: "IS NOT NULL", Value: ""}, false},

		// Invalid
		{"invalid", storage.WhereCondition{}, true},
		{"", storage.WhereCondition{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseWhereClause(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWhereClause(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Field != tt.expected.Field {
					t.Errorf("parseWhereClause(%q) Field = %q, want %q", tt.input, got.Field, tt.expected.Field)
				}
				if got.Operator != tt.expected.Operator {
					t.Errorf("parseWhereClause(%q) Operator = %q, want %q", tt.input, got.Operator, tt.expected.Operator)
				}
				if got.Value != tt.expected.Value {
					t.Errorf("parseWhereClause(%q) Value = %q, want %q", tt.input, got.Value, tt.expected.Value)
				}
			}
		})
	}
}

// TestListRecordsStatus tests that status column shows correctly
func TestListRecordsStatus(t *testing.T) {
	t.Run("shows deleted status for deleted records", func(t *testing.T) {
		// Given: Stash with a deleted record
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "ToDelete"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash list --deleted`
		rootCmd.SetArgs([]string{"list", "--deleted"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Shows "deleted" status
		if !strings.Contains(output, "deleted") {
			t.Error("expected 'deleted' status to be shown")
		}
	})
}
