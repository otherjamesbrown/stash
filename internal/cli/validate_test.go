package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// TestValidationTypes tests the validation type checking
func TestValidationTypes(t *testing.T) {
	t.Run("valid validation types", func(t *testing.T) {
		validTypes := []string{"email", "url", "number", "date"}
		for _, vt := range validTypes {
			if !IsValidValidationType(vt) {
				t.Errorf("expected '%s' to be a valid validation type", vt)
			}
		}
	})

	t.Run("invalid validation types", func(t *testing.T) {
		invalidTypes := []string{"invalid", "Email", "URL", "int", "string", ""}
		for _, vt := range invalidTypes {
			if IsValidValidationType(vt) {
				t.Errorf("expected '%s' to be an invalid validation type", vt)
			}
		}
	})
}

// TestEmailValidation tests email format validation
func TestEmailValidation(t *testing.T) {
	col := &model.Column{Name: "email", Validate: "email"}

	t.Run("valid emails", func(t *testing.T) {
		validEmails := []string{
			"test@example.com",
			"user.name@domain.org",
			"user+tag@example.co.uk",
			"a@b.co",
		}
		for _, email := range validEmails {
			result := ValidateValue(col, email)
			if !result.Valid {
				t.Errorf("expected email '%s' to be valid, got errors: %v", email, result.Errors)
			}
		}
	})

	t.Run("invalid emails", func(t *testing.T) {
		invalidEmails := []string{
			"notanemail",
			"@nodomain.com",
			"noat.com",
			"spaces in@email.com",
			"",
		}
		for _, email := range invalidEmails {
			// Skip empty - that's handled by required
			if email == "" {
				continue
			}
			result := ValidateValue(col, email)
			if result.Valid {
				t.Errorf("expected email '%s' to be invalid", email)
			}
		}
	})
}

// TestURLValidation tests URL format validation
func TestURLValidation(t *testing.T) {
	col := &model.Column{Name: "website", Validate: "url"}

	t.Run("valid URLs", func(t *testing.T) {
		validURLs := []string{
			"https://example.com",
			"http://localhost:8080",
			"https://sub.domain.org/path?query=1",
			"ftp://files.example.com",
		}
		for _, url := range validURLs {
			result := ValidateValue(col, url)
			if !result.Valid {
				t.Errorf("expected URL '%s' to be valid, got errors: %v", url, result.Errors)
			}
		}
	})

	t.Run("invalid URLs", func(t *testing.T) {
		invalidURLs := []string{
			"notaurl",
			"example.com", // missing scheme
			"://noscheme.com",
			"http://", // missing host
		}
		for _, url := range invalidURLs {
			result := ValidateValue(col, url)
			if result.Valid {
				t.Errorf("expected URL '%s' to be invalid", url)
			}
		}
	})
}

// TestNumberValidation tests number format validation
func TestNumberValidation(t *testing.T) {
	col := &model.Column{Name: "price", Validate: "number"}

	t.Run("valid numbers", func(t *testing.T) {
		validNumbers := []string{
			"123",
			"-456",
			"0",
			"3.14",
			"-2.5",
			"1000000",
		}
		for _, num := range validNumbers {
			result := ValidateValue(col, num)
			if !result.Valid {
				t.Errorf("expected number '%s' to be valid, got errors: %v", num, result.Errors)
			}
		}
	})

	t.Run("invalid numbers", func(t *testing.T) {
		invalidNumbers := []string{
			"abc",
			"12.34.56",
			"1,000",
			"$100",
		}
		for _, num := range invalidNumbers {
			result := ValidateValue(col, num)
			if result.Valid {
				t.Errorf("expected number '%s' to be invalid", num)
			}
		}
	})
}

// TestDateValidation tests date format validation
func TestDateValidation(t *testing.T) {
	col := &model.Column{Name: "created", Validate: "date"}

	t.Run("valid dates", func(t *testing.T) {
		validDates := []string{
			"2024-01-15",
			"2024-12-31T23:59:59Z",
			"2024-06-15T10:30:00",
		}
		for _, date := range validDates {
			result := ValidateValue(col, date)
			if !result.Valid {
				t.Errorf("expected date '%s' to be valid, got errors: %v", date, result.Errors)
			}
		}
	})

	t.Run("invalid dates", func(t *testing.T) {
		invalidDates := []string{
			"not a date",
			"01/15/2024",
			"2024-13-01", // invalid month
			"Jan 15, 2024",
		}
		for _, date := range invalidDates {
			result := ValidateValue(col, date)
			if result.Valid {
				t.Errorf("expected date '%s' to be invalid", date)
			}
		}
	})
}

// TestEnumValidation tests enum constraint validation
func TestEnumValidation(t *testing.T) {
	col := &model.Column{
		Name: "status",
		Enum: []string{"pending", "active", "closed"},
	}

	t.Run("valid enum values", func(t *testing.T) {
		validValues := []string{"pending", "active", "closed"}
		for _, val := range validValues {
			result := ValidateValue(col, val)
			if !result.Valid {
				t.Errorf("expected enum value '%s' to be valid, got errors: %v", val, result.Errors)
			}
		}
	})

	t.Run("invalid enum values", func(t *testing.T) {
		invalidValues := []string{"invalid", "PENDING", "Active", "unknown"}
		for _, val := range invalidValues {
			result := ValidateValue(col, val)
			if result.Valid {
				t.Errorf("expected enum value '%s' to be invalid", val)
			}
			if len(result.Errors) > 0 && result.Errors[0].Rule != "enum" {
				t.Errorf("expected rule 'enum', got '%s'", result.Errors[0].Rule)
			}
		}
	})
}

// TestRequiredValidation tests required constraint validation
func TestRequiredValidation(t *testing.T) {
	col := &model.Column{Name: "name", Required: true}

	t.Run("valid non-empty value", func(t *testing.T) {
		result := ValidateValue(col, "John")
		if !result.Valid {
			t.Errorf("expected non-empty value to be valid")
		}
	})

	t.Run("invalid empty values", func(t *testing.T) {
		emptyValues := []interface{}{nil, ""}
		for _, val := range emptyValues {
			result := ValidateValue(col, val)
			if result.Valid {
				t.Errorf("expected empty value to be invalid for required field")
			}
			if len(result.Errors) > 0 && result.Errors[0].Rule != "required" {
				t.Errorf("expected rule 'required', got '%s'", result.Errors[0].Rule)
			}
		}
	})
}

// TestColumnAddWithValidation tests adding columns with validation constraints
func TestColumnAddWithValidation(t *testing.T) {
	t.Run("add column with email validation", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create stash
		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		// Add column with email validation
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"column", "add", "email", "--validate", "email"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify column was created with validation
		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("test")
		col := stash.Columns.Find("email")
		if col == nil {
			t.Fatal("expected email column to exist")
		}
		if col.Validate != "email" {
			t.Errorf("expected validate='email', got '%s'", col.Validate)
		}
	})

	t.Run("add column with enum", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"column", "add", "status", "--enum", "pending,active,closed"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("test")
		col := stash.Columns.Find("status")
		if col == nil {
			t.Fatal("expected status column to exist")
		}
		if len(col.Enum) != 3 {
			t.Errorf("expected 3 enum values, got %d", len(col.Enum))
		}
	})

	t.Run("add column with required flag", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"column", "add", "name", "--required"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		defer store.Close()

		stash, _ := store.GetStash("test")
		col := stash.Columns.Find("name")
		if col == nil {
			t.Fatal("expected name column to exist")
		}
		if !col.Required {
			t.Error("expected required=true")
		}
	})

	t.Run("reject invalid validation type", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"column", "add", "field", "--validate", "invalid"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for invalid validation type, got %d", ExitCode)
		}
	})
}

// TestAddRecordWithValidation tests adding records with validation constraints
func TestAddRecordWithValidation(t *testing.T) {
	t.Run("add record with valid email", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Setup stash with email column
		rootCmd.SetArgs([]string{"init", "contacts", "--prefix", "con-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		col := model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		}
		store.AddColumn("contacts", col)
		store.Close()

		// Add record with valid email
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "test@example.com"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("reject record with invalid email", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "contacts", "--prefix", "con-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		col := model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		}
		store.AddColumn("contacts", col)
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "invalid-email"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for validation error, got %d", ExitCode)
		}
	})

	t.Run("add record with valid enum value", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "tasks", "--prefix", "tsk-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		col := model.Column{
			Name:    "status",
			Enum:    []string{"pending", "active", "closed"},
			Added:   time.Now(),
			AddedBy: "test",
		}
		store.AddColumn("tasks", col)
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "pending"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("reject record with invalid enum value", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "tasks", "--prefix", "tsk-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		col := model.Column{
			Name:    "status",
			Enum:    []string{"pending", "active", "closed"},
			Added:   time.Now(),
			AddedBy: "test",
		}
		store.AddColumn("tasks", col)
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "invalid_status"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for validation error, got %d", ExitCode)
		}
	})

	t.Run("reject record missing required field", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "items", "--prefix", "itm-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		// Add primary column
		store.AddColumn("items", model.Column{
			Name:    "name",
			Added:   time.Now(),
			AddedBy: "test",
		})
		// Add required column
		store.AddColumn("items", model.Column{
			Name:     "price",
			Required: true,
			Added:    time.Now(),
			AddedBy:  "test",
		})
		store.Close()

		// Add record without setting required price field
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Test Item"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for missing required field, got %d", ExitCode)
		}
	})

	t.Run("add record with required field provided", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "items", "--prefix", "itm-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("items", model.Column{
			Name:    "name",
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.AddColumn("items", model.Column{
			Name:     "price",
			Required: true,
			Validate: "number",
			Added:    time.Now(),
			AddedBy:  "test",
		})
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"add", "Test Item", "--set", "price=99.99"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})
}

// TestSetRecordWithValidation tests updating records with validation constraints
func TestSetRecordWithValidation(t *testing.T) {
	t.Run("set with valid email", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Setup
		rootCmd.SetArgs([]string{"init", "contacts", "--prefix", "con-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("contacts", model.Column{
			Name:    "name",
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.AddColumn("contacts", model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		})
		store.Close()

		// Add record
		resetFlags()
		rootCmd.SetArgs([]string{"add", "John"})
		rootCmd.Execute()

		// Get record ID
		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("contacts", storage.ListOptions{ParentID: "*"})
		store.Close()
		recordID := records[0].ID

		// Update with valid email
		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "email=john@example.com"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
	})

	t.Run("reject set with invalid email", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "contacts", "--prefix", "con-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("contacts", model.Column{
			Name:    "name",
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.AddColumn("contacts", model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		})
		store.Close()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "John"})
		rootCmd.Execute()

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("contacts", storage.ListOptions{ParentID: "*"})
		store.Close()
		recordID := records[0].ID

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "email=invalid"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for validation error, got %d", ExitCode)
		}
	})

	t.Run("reject set with invalid enum value", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "tasks", "--prefix", "tsk-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("tasks", model.Column{
			Name:    "title",
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.AddColumn("tasks", model.Column{
			Name:    "status",
			Enum:    []string{"pending", "active", "closed"},
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.Close()

		resetFlags()
		rootCmd.SetArgs([]string{"add", "Task 1"})
		rootCmd.Execute()

		store, _ = storage.NewStore(filepath.Join(tempDir, ".stash"))
		records, _ := store.ListRecords("tasks", storage.ListOptions{ParentID: "*"})
		store.Close()
		recordID := records[0].ID

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"set", recordID, "status=invalid_status"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for enum validation error, got %d", ExitCode)
		}
	})
}

// TestValidateCommand tests the stash validate command
func TestValidateCommand(t *testing.T) {
	t.Run("validate with no errors", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("test", model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		})

		// Add valid record directly
		record := &model.Record{
			ID:        "tst-0001",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedBy: "test",
			Fields:    map[string]interface{}{"email": "test@example.com"},
		}
		store.CreateRecord("test", record)
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"validate"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0 for valid records, got %d", ExitCode)
		}
	})

	t.Run("validate with errors returns exit code 2", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("test", model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		})

		// Add invalid record directly (bypassing validation)
		record := &model.Record{
			ID:        "tst-0001",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedBy: "test",
			Fields:    map[string]interface{}{"email": "invalid-email"},
		}
		store.CreateRecord("test", record)
		store.Close()

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"validate"})
		rootCmd.Execute()

		if ExitCode != 2 {
			t.Errorf("expected exit code 2 for validation errors, got %d", ExitCode)
		}
	})

	t.Run("validate JSON output", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("test", model.Column{
			Name:     "email",
			Validate: "email",
			Added:    time.Now(),
			AddedBy:  "test",
		})

		record := &model.Record{
			ID:        "tst-0001",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			CreatedBy: "test",
			UpdatedBy: "test",
			Fields:    map[string]interface{}{"email": "invalid-email"},
		}
		store.CreateRecord("test", record)
		store.Close()

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"validate", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Parse JSON output
		var result ValidateStashOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		if result.ErrorCount == 0 {
			t.Error("expected error_count > 0")
		}
		if len(result.Errors) == 0 {
			t.Error("expected errors array to be non-empty")
		}
	})
}

// TestColumnListShowsValidation tests that column list shows validation info
func TestColumnListShowsValidation(t *testing.T) {
	t.Run("column list JSON includes validation fields", func(t *testing.T) {
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "tst-"})
		rootCmd.Execute()

		store, _ := storage.NewStore(filepath.Join(tempDir, ".stash"))
		store.AddColumn("test", model.Column{
			Name:     "email",
			Validate: "email",
			Required: true,
			Added:    time.Now(),
			AddedBy:  "test",
		})
		store.AddColumn("test", model.Column{
			Name:    "status",
			Enum:    []string{"pending", "active"},
			Added:   time.Now(),
			AddedBy: "test",
		})
		store.Close()

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		ExitCode = 0
		resetFlags()
		rootCmd.SetArgs([]string{"column", "list", "--json"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		var columns []ColumnInfo
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &columns); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		// Find email column
		var emailCol *ColumnInfo
		for i := range columns {
			if columns[i].Name == "email" {
				emailCol = &columns[i]
				break
			}
		}

		if emailCol == nil {
			t.Fatal("expected email column in output")
		}
		if emailCol.Validate != "email" {
			t.Errorf("expected validate='email', got '%s'", emailCol.Validate)
		}
		if !emailCol.Required {
			t.Error("expected required=true for email column")
		}

		// Find status column
		var statusCol *ColumnInfo
		for i := range columns {
			if columns[i].Name == "status" {
				statusCol = &columns[i]
				break
			}
		}

		if statusCol == nil {
			t.Fatal("expected status column in output")
		}
		if len(statusCol.Enum) != 2 {
			t.Errorf("expected 2 enum values, got %d", len(statusCol.Enum))
		}
	})
}
