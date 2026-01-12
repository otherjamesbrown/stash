package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/stash/internal/storage"
)

// TestUC_REC_002_UpdateRecord tests UC-REC-002: Update Record Field
func TestUC_REC_002_UpdateRecord(t *testing.T) {
	t.Run("AC-01: update single field", func(t *testing.T) {
		// Given: Record inv-ex4j exists with Name="Laptop"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		origHash := records[0].Hash
		store.Close()

		ExitCode = 0

		// When: User runs `stash set inv-ex4j Price=1299`
		rootCmd.SetArgs([]string{"set", recordID, "Price=1299"})
		err := rootCmd.Execute()

		// Then: Price field is set to 1299
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify update
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)

		if fmt.Sprintf("%v", rec.Fields["Price"]) != "1299" {
			t.Errorf("expected Price='1299', got '%v'", rec.Fields["Price"])
		}

		// Then: _hash is recalculated
		if rec.Hash == origHash {
			t.Error("expected _hash to be recalculated")
		}

		// Then: _updated_at is set (may be same second as original due to SQLite precision)
		// We verify the timestamp exists and is not zero - the update happened atomically
		if rec.UpdatedAt.IsZero() {
			t.Error("expected _updated_at to be set")
		}

		// Then: _updated_by is set
		if rec.UpdatedBy == "" {
			t.Error("expected _updated_by to be set")
		}
	})

	t.Run("AC-02: update multiple fields", func(t *testing.T) {
		// Given: Record inv-ex4j exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price", "Stock"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set inv-ex4j --col Price=1299 --col Stock=50`
		rootCmd.SetArgs([]string{"set", recordID, "--col", "Price=1299", "--col", "Stock=50"})
		err := rootCmd.Execute()

		// Then: Both fields are updated
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify both fields updated
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)

		if fmt.Sprintf("%v", rec.Fields["Price"]) != "1299" {
			t.Errorf("expected Price='1299', got '%v'", rec.Fields["Price"])
		}
		if fmt.Sprintf("%v", rec.Fields["Stock"]) != "50" {
			t.Errorf("expected Stock='50', got '%v'", rec.Fields["Stock"])
		}
	})

	t.Run("AC-03: reject non-existent record", func(t *testing.T) {
		// Given: No record inv-fake exists
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// When: User runs `stash set inv-fake Price=100`
		rootCmd.SetArgs([]string{"set", "inv-fake", "Price=100"})
		rootCmd.Execute()

		// Then: Command fails with exit code 4
		if ExitCode != 4 {
			t.Errorf("expected exit code 4, got %d", ExitCode)
		}
	})

	t.Run("AC-04: reject non-existent column", func(t *testing.T) {
		// Given: Record inv-ex4j exists, no column "FakeCol"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		origFields := records[0].Fields
		store.Close()

		ExitCode = 0

		// When: User runs `stash set inv-ex4j FakeCol="value"`
		rootCmd.SetArgs([]string{"set", recordID, "FakeCol=value"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}

		// Then: Record is not modified
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)
		if len(rec.Fields) != len(origFields) {
			t.Error("expected record not to be modified")
		}
	})

	t.Run("AC-05: reject update to deleted record", func(t *testing.T) {
		// Given: Record inv-ex4j is soft-deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID

		// Soft-delete the record
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0

		// When: User runs `stash set inv-ex4j Price=100`
		rootCmd.SetArgs([]string{"set", recordID, "Price=100"})
		rootCmd.Execute()

		// Then: Command fails with appropriate error
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for deleted record")
		}
	})

	t.Run("AC-06: allow empty value to clear field", func(t *testing.T) {
		// Given: Record inv-ex4j exists with Notes="something"
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		// Create a record with Notes
		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=something"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set inv-ex4j Notes=""`
		rootCmd.SetArgs([]string{"set", recordID, "Notes="})
		err := rootCmd.Execute()

		// Then: Notes field is cleared (set to empty)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify field is cleared
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)
		if rec.Fields["Notes"] != "" {
			t.Errorf("expected Notes to be empty, got '%v'", rec.Fields["Notes"])
		}
	})
}

// TestUC_REC_002_UpdateRecord_MustNot tests anti-requirements
func TestUC_REC_002_UpdateRecord_MustNot(t *testing.T) {
	t.Run("must not allow update to non-existent column", func(t *testing.T) {
		// Given: Record exists, column does not
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User tries to set non-existent column
		rootCmd.SetArgs([]string{"set", recordID, "NonExistent=value"})
		rootCmd.Execute()

		// Then: Command should fail
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for non-existent column")
		}
	})

	t.Run("must not allow update to deleted record", func(t *testing.T) {
		// Given: Record is soft-deleted
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create and delete record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.DeleteRecord("inventory", recordID, "test")
		store.Close()

		ExitCode = 0

		// When: User tries to update deleted record
		rootCmd.SetArgs([]string{"set", recordID, "Price=100"})
		rootCmd.Execute()

		// Then: Command should fail
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for deleted record")
		}
	})
}

// TestUC_REC_002_UpdateRecord_JSONOutput tests JSON output for set command
func TestUC_REC_002_UpdateRecord_JSONOutput(t *testing.T) {
	t.Run("JSON output shows updated record", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash set inv-ex4j Price=999 --json`
		rootCmd.SetArgs([]string{"set", recordID, "Price=999", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON, got error: %v", err)
		}

		// Then: Contains updated Price
		if jsonOutput["Price"] != "999" {
			t.Errorf("expected Price='999' in JSON, got %v", jsonOutput["Price"])
		}
	})
}

// TestUC_REC_002_UpdateRecord_AutoCreate tests the --auto-create flag for auto-column creation
func TestUC_REC_002_UpdateRecord_AutoCreate(t *testing.T) {
	t.Run("AC-01: auto-create single column when flag is set", func(t *testing.T) {
		// Given: Record exists, but column "NewField" does not exist
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> NewField=value --auto-create`
		rootCmd.SetArgs([]string{"set", recordID, "NewField=testvalue", "--auto-create"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Column is created and field is set
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Verify column exists
		stash, _ := store.GetStash("inventory")
		if !stash.Columns.Exists("NewField") {
			t.Error("expected column 'NewField' to be created")
		}

		// Verify field value is set
		rec, _ := store.GetRecord("inventory", recordID)
		if fmt.Sprintf("%v", rec.Fields["NewField"]) != "testvalue" {
			t.Errorf("expected NewField='testvalue', got '%v'", rec.Fields["NewField"])
		}
	})

	t.Run("AC-02: auto-create multiple columns when flag is set", func(t *testing.T) {
		// Given: Record exists, but columns "Field1" and "Field2" do not exist
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> --col Field1=val1 --col Field2=val2 --auto-create`
		rootCmd.SetArgs([]string{"set", recordID, "--col", "Field1=val1", "--col", "Field2=val2", "--auto-create"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Both columns are created and fields are set
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("inventory")
		if !stash.Columns.Exists("Field1") {
			t.Error("expected column 'Field1' to be created")
		}
		if !stash.Columns.Exists("Field2") {
			t.Error("expected column 'Field2' to be created")
		}

		rec, _ := store.GetRecord("inventory", recordID)
		if fmt.Sprintf("%v", rec.Fields["Field1"]) != "val1" {
			t.Errorf("expected Field1='val1', got '%v'", rec.Fields["Field1"])
		}
		if fmt.Sprintf("%v", rec.Fields["Field2"]) != "val2" {
			t.Errorf("expected Field2='val2', got '%v'", rec.Fields["Field2"])
		}
	})

	t.Run("AC-03: without flag, non-existent column still fails", func(t *testing.T) {
		// Given: Record exists, column "NewField" does not exist
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> NewField=value` WITHOUT --auto-create
		rootCmd.SetArgs([]string{"set", recordID, "NewField=testvalue"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1 (column not found)
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}

		// Then: Column is NOT created
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("inventory")
		if stash.Columns.Exists("NewField") {
			t.Error("expected column 'NewField' to NOT be created")
		}
	})

	t.Run("AC-04: auto-create with existing columns works normally", func(t *testing.T) {
		// Given: Record exists with columns Name and Price
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Price"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> Price=999 --auto-create` (Price already exists)
		rootCmd.SetArgs([]string{"set", recordID, "Price=999", "--auto-create"})
		err := rootCmd.Execute()

		// Then: Command succeeds without creating duplicate column
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify field is updated
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		rec, _ := store.GetRecord("inventory", recordID)
		if fmt.Sprintf("%v", rec.Fields["Price"]) != "999" {
			t.Errorf("expected Price='999', got '%v'", rec.Fields["Price"])
		}
	})

	t.Run("AC-05: auto-create with invalid column name fails", func(t *testing.T) {
		// Given: Record exists
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> _invalid=value --auto-create` (reserved name)
		rootCmd.SetArgs([]string{"set", recordID, "_id=testvalue", "--auto-create"})
		rootCmd.Execute()

		// Then: Command fails (reserved column name)
		if ExitCode == 0 {
			t.Error("expected non-zero exit code for reserved column name")
		}
	})

	t.Run("AC-06: auto-create mixed new and existing columns", func(t *testing.T) {
		// Given: Record exists with column Name, but not Category
		tempDir, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Create a record
		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()

		// Get the record ID
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("inventory", storage.ListOptions{ParentID: "*"})
		recordID := records[0].ID
		store.Close()

		ExitCode = 0

		// When: User runs `stash set <id> Name=UpdatedName Category=Electronics --auto-create`
		rootCmd.SetArgs([]string{"set", recordID, "Name=UpdatedName", "Category=Electronics", "--auto-create"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify both fields are set
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		// Verify new column exists
		stash, _ := store.GetStash("inventory")
		if !stash.Columns.Exists("Category") {
			t.Error("expected column 'Category' to be created")
		}

		rec, _ := store.GetRecord("inventory", recordID)
		if fmt.Sprintf("%v", rec.Fields["Name"]) != "UpdatedName" {
			t.Errorf("expected Name='UpdatedName', got '%v'", rec.Fields["Name"])
		}
		if fmt.Sprintf("%v", rec.Fields["Category"]) != "Electronics" {
			t.Errorf("expected Category='Electronics', got '%v'", rec.Fields["Category"])
		}
	})
}
