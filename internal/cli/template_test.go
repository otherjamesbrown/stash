package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetTemplateFlags resets template and global flags between tests
func resetTemplateFlags() {
	templateDesc = ""
	jsonOutput = false
	stashName = ""
	actorName = ""
	queryCSV = false
	queryNoHeaders = false
	queryColumns = ""
	// Reset add command flags
	addSetFlags = nil
	addParentID = ""
	// Reset column command flags
	columnDesc = ""
}

// TestTemplateSave tests the template save command
func TestTemplateSave(t *testing.T) {
	t.Run("save template successfully", func(t *testing.T) {
		// Given: A stash exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("failed to create stash: %v", err)
		}

		// When: User saves a template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "high-priority", "SELECT * FROM inventory WHERE priority='high'"})
		err := rootCmd.Execute()

		// Then: Template is saved successfully
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify template file exists
		templatesPath := filepath.Join(tempDir, ".stash", "templates.json")
		data, err := os.ReadFile(templatesPath)
		if err != nil {
			t.Fatalf("failed to read templates.json: %v", err)
		}

		var templates []map[string]interface{}
		if err := json.Unmarshal(data, &templates); err != nil {
			t.Fatalf("failed to parse templates.json: %v", err)
		}

		if len(templates) != 1 {
			t.Fatalf("expected 1 template, got %d", len(templates))
		}

		if templates[0]["name"] != "high-priority" {
			t.Errorf("expected template name 'high-priority', got %v", templates[0]["name"])
		}
		if templates[0]["query"] != "SELECT * FROM inventory WHERE priority='high'" {
			t.Errorf("unexpected query: %v", templates[0]["query"])
		}
	})

	t.Run("save template with description", func(t *testing.T) {
		// Given: A stash exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User saves a template with description
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "needs-review", "SELECT * FROM inventory WHERE status='pending'", "--desc", "Items needing review"})
		err := rootCmd.Execute()

		// Then: Template is saved with description
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		templatesPath := filepath.Join(tempDir, ".stash", "templates.json")
		data, _ := os.ReadFile(templatesPath)

		var templates []map[string]interface{}
		json.Unmarshal(data, &templates)

		if templates[0]["desc"] != "Items needing review" {
			t.Errorf("expected description 'Items needing review', got %v", templates[0]["desc"])
		}
	})

	t.Run("reject duplicate template name", func(t *testing.T) {
		// Given: A template already exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "my-template", "SELECT * FROM inventory"})
		rootCmd.Execute()

		// When: User tries to save template with same name
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "my-template", "SELECT id FROM inventory"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("reject invalid template name - starts with number", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to save template with invalid name
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "123template", "SELECT * FROM inventory"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("reject invalid template name - contains spaces", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to save template with spaces in name
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "my template", "SELECT * FROM inventory"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("accept valid template names with hyphens and underscores", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		validNames := []string{"my-template", "my_template", "myTemplate123", "Template_with-Mixed123"}
		for _, name := range validNames {
			ExitCode = 0
			rootCmd.SetArgs([]string{"template", "save", name, "SELECT * FROM inventory"})
			err := rootCmd.Execute()

			if err != nil {
				t.Errorf("expected no error for name %q, got %v", name, err)
			}
			if ExitCode != 0 {
				t.Errorf("expected exit code 0 for name %q, got %d", name, ExitCode)
			}
		}
	})

	t.Run("reject non-SELECT query", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to save template with non-SELECT query
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "bad-query", "DELETE FROM inventory"})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("reject empty query", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to save template with empty query
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "empty-query", ""})
		rootCmd.Execute()

		// Then: Command fails with exit code 2
		if ExitCode != 2 {
			t.Errorf("expected exit code 2, got %d", ExitCode)
		}
	})

	t.Run("JSON output on save", func(t *testing.T) {
		// Given: A stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User saves template with --json
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "save", "test-template", "SELECT * FROM inventory", "--json"})
		rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["name"] != "test-template" {
			t.Errorf("expected name 'test-template', got %v", result["name"])
		}
	})
}

// TestTemplateRun tests the template run command
func TestTemplateRun(t *testing.T) {
	t.Run("run template successfully", func(t *testing.T) {
		// Given: A stash with data and a template exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetTemplateFlags()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"column", "add", "Name", "Priority"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Widget", "--set", "Priority=high"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"template", "save", "all-items", "SELECT * FROM inventory"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs the template
		rootCmd.SetArgs([]string{"template", "run", "all-items", "--json"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 16384)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Reset flags after test
		resetTemplateFlags()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output contains query results
		var results []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &results); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("run non-existent template", func(t *testing.T) {
		// Given: A stash exists but no template
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to run non-existent template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "run", "does-not-exist"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})

	t.Run("run template with CSV output", func(t *testing.T) {
		// Given: A stash with data and a template exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetTemplateFlags()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"column", "add", "Name"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"add", "Widget"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		rootCmd.SetArgs([]string{"template", "save", "all-items", "SELECT Name FROM inventory"})
		rootCmd.Execute()
		resetTemplateFlags()
		ExitCode = 0

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs the template with --csv
		rootCmd.SetArgs([]string{"template", "run", "all-items", "--csv"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Reset flags after test
		resetTemplateFlags()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output is CSV format
		if !strings.Contains(output, "Name") {
			t.Errorf("expected CSV header with 'Name', got: %s", output)
		}
	})
}

// TestTemplateList tests the template list command
func TestTemplateList(t *testing.T) {
	t.Run("list templates successfully", func(t *testing.T) {
		// Given: A stash with templates exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "template-1", "SELECT * FROM inventory"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "template-2", "SELECT id FROM inventory", "--desc", "IDs only"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User lists templates
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "list", "--json"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output contains both templates
		var templates []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &templates); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		if len(templates) != 2 {
			t.Errorf("expected 2 templates, got %d", len(templates))
		}
	})

	t.Run("list empty templates", func(t *testing.T) {
		// Given: A stash with no templates
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User lists templates
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "list", "--json"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output is empty array
		var templates []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &templates); err != nil {
			t.Fatalf("expected valid JSON array, got error: %v\nOutput: %s", err, output)
		}

		if len(templates) != 0 {
			t.Errorf("expected 0 templates, got %d", len(templates))
		}
	})
}

// TestTemplateShow tests the template show command
func TestTemplateShow(t *testing.T) {
	t.Run("show template successfully", func(t *testing.T) {
		// Given: A template exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "my-template", "SELECT * FROM inventory", "--desc", "My description"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User shows the template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "show", "my-template", "--json"})
		err := rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Output contains template details
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["name"] != "my-template" {
			t.Errorf("expected name 'my-template', got %v", result["name"])
		}
		if result["desc"] != "My description" {
			t.Errorf("expected desc 'My description', got %v", result["desc"])
		}
		if result["query"] != "SELECT * FROM inventory" {
			t.Errorf("expected query 'SELECT * FROM inventory', got %v", result["query"])
		}
		if result["created_at"] == nil {
			t.Error("expected created_at to be set")
		}
		if result["created_by"] == nil {
			t.Error("expected created_by to be set")
		}
	})

	t.Run("show non-existent template", func(t *testing.T) {
		// Given: A stash exists but no template
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to show non-existent template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "show", "does-not-exist"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})
}

// TestTemplateRm tests the template rm command
func TestTemplateRm(t *testing.T) {
	t.Run("delete template successfully", func(t *testing.T) {
		// Given: A template exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "to-delete", "SELECT * FROM inventory"})
		rootCmd.Execute()

		// When: User deletes the template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "rm", "to-delete"})
		err := rootCmd.Execute()

		// Then: Command succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Then: Template is removed from file
		templatesPath := filepath.Join(tempDir, ".stash", "templates.json")
		data, _ := os.ReadFile(templatesPath)

		var templates []map[string]interface{}
		json.Unmarshal(data, &templates)

		if len(templates) != 0 {
			t.Errorf("expected 0 templates, got %d", len(templates))
		}
	})

	t.Run("delete non-existent template", func(t *testing.T) {
		// Given: A stash exists but no template
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// When: User tries to delete non-existent template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "rm", "does-not-exist"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})

	t.Run("delete preserves other templates", func(t *testing.T) {
		// Given: Multiple templates exist
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "keep-1", "SELECT * FROM inventory"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "to-delete", "SELECT id FROM inventory"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "keep-2", "SELECT Name FROM inventory"})
		rootCmd.Execute()

		// When: User deletes one template
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "rm", "to-delete"})
		rootCmd.Execute()

		// Then: Other templates remain
		templatesPath := filepath.Join(tempDir, ".stash", "templates.json")
		data, _ := os.ReadFile(templatesPath)

		var templates []map[string]interface{}
		json.Unmarshal(data, &templates)

		if len(templates) != 2 {
			t.Errorf("expected 2 templates, got %d", len(templates))
		}

		// Verify the correct templates remain
		names := make(map[string]bool)
		for _, t := range templates {
			names[t["name"].(string)] = true
		}

		if !names["keep-1"] {
			t.Error("expected 'keep-1' to remain")
		}
		if !names["keep-2"] {
			t.Error("expected 'keep-2' to remain")
		}
		if names["to-delete"] {
			t.Error("expected 'to-delete' to be removed")
		}
	})

	t.Run("JSON output on delete", func(t *testing.T) {
		// Given: A template exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"template", "save", "to-delete", "SELECT * FROM inventory"})
		rootCmd.Execute()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User deletes template with --json
		ExitCode = 0
		rootCmd.SetArgs([]string{"template", "rm", "to-delete", "--json"})
		rootCmd.Execute()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if result["deleted"] != true {
			t.Errorf("expected deleted=true, got %v", result["deleted"])
		}
		if result["name"] != "to-delete" {
			t.Errorf("expected name 'to-delete', got %v", result["name"])
		}
	})
}

// TestTemplateValidation tests template name validation
func TestTemplateValidation(t *testing.T) {
	validNames := []string{
		"a",
		"abc",
		"my-template",
		"my_template",
		"myTemplate",
		"Template123",
		"a-b-c-d",
		"a_b_c_d",
		"aBcDeF",
	}

	invalidNames := []string{
		"",
		"123",
		"123abc",
		"-template",
		"_template",
		"my template",
		"my.template",
		"my@template",
		"my/template",
	}

	for _, name := range validNames {
		err := validateTemplateName(name)
		if err != nil {
			t.Errorf("expected name %q to be valid, got error: %v", name, err)
		}
	}

	for _, name := range invalidNames {
		err := validateTemplateName(name)
		if err == nil {
			t.Errorf("expected name %q to be invalid, but it was accepted", name)
		}
	}
}

// TestTemplateNoStashDir tests error handling when no .stash directory exists
func TestTemplateNoStashDir(t *testing.T) {
	commands := [][]string{
		{"template", "save", "test", "SELECT * FROM t"},
		{"template", "run", "test"},
		{"template", "list"},
		{"template", "show", "test"},
		{"template", "rm", "test"},
	}

	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// Given: No .stash directory exists
			_, cleanup := setupTestEnv(t)
			defer cleanup()

			// When: User runs template command
			ExitCode = 0
			rootCmd.SetArgs(args)
			rootCmd.Execute()

			// Then: Command fails with exit code 1
			if ExitCode != 1 {
				t.Errorf("expected exit code 1, got %d", ExitCode)
			}
		})
	}
}
