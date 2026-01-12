package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSearchCommand tests the stash search command
func TestSearchCommand(t *testing.T) {
	t.Run("search across all text fields", func(t *testing.T) {
		// Given: Stash with records containing searchable content
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description", "Industry"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment company", "--set", "Industry=Media"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Description=Technology company", "--set", "Industry=Tech"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Netflix", "--set", "Description=Streaming service", "--set", "Industry=Media"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "disney"`
		rootCmd.SetArgs([]string{"search", "disney"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records containing "disney" are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "Disney") {
			t.Error("expected Disney to be shown")
		}
		if strings.Contains(output, "Apple") {
			t.Error("expected Apple to NOT be shown")
		}
		if strings.Contains(output, "Netflix") {
			t.Error("expected Netflix to NOT be shown")
		}
	})

	t.Run("search is case-insensitive", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Description=Technology company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "DISNEY"` (uppercase)
		rootCmd.SetArgs([]string{"search", "DISNEY"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Case-insensitive match works
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Disney") {
			t.Errorf("expected Disney to be shown with case-insensitive search, output: %s", output)
		}
	})

	t.Run("search with --in flag for specific column", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description", "Notes"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Media company", "--set", "Notes=Good investment"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Media Corp", "--set", "Description=News organization", "--set", "Notes=Declining"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "media" --in Name`
		rootCmd.SetArgs([]string{"search", "media", "--in", "Name"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Only records with "media" in Name are shown (not Description)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Media Corp") {
			t.Error("expected 'Media Corp' to be shown (matches in Name)")
		}
		if strings.Contains(output, "Disney") {
			t.Error("expected 'Disney' to NOT be shown (media is only in Description)")
		}
	})

	t.Run("search with multiple --in flags", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description", "Notes"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment", "--set", "Notes=Media company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Media Corp", "--set", "Description=News organization", "--set", "Notes=Declining"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Description=Tech company", "--set", "Notes=Growing"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "media" --in Name --in Notes`
		rootCmd.SetArgs([]string{"search", "media", "--in", "Name", "--in", "Notes"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records with "media" in Name OR Notes are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Disney") {
			t.Error("expected 'Disney' to be shown (media in Notes)")
		}
		if !strings.Contains(output, "Media Corp") {
			t.Error("expected 'Media Corp' to be shown (media in Name)")
		}
		if strings.Contains(output, "Apple") {
			t.Error("expected 'Apple' to NOT be shown")
		}
	})

	t.Run("search with JSON output", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Description=Technology company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "disney" --json`
		rootCmd.SetArgs([]string{"search", "disney", "--json"})
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

		var records []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &records); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		if len(records) != 1 {
			t.Errorf("expected 1 record, got %d", len(records))
		}
		if len(records) > 0 && records[0]["Name"] != "Disney" {
			t.Errorf("expected Disney, got %v", records[0]["Name"])
		}
	})

	t.Run("search finds matches in description field", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment and media company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple", "--set", "Description=Technology hardware company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "entertainment"`
		rootCmd.SetArgs([]string{"search", "entertainment"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Records with "entertainment" in any field are shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Disney") {
			t.Error("expected Disney to be shown (entertainment in Description)")
		}
		if strings.Contains(output, "Apple") {
			t.Error("expected Apple to NOT be shown")
		}
	})

	t.Run("search with no results", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney", "--set", "Description=Entertainment company"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "nonexistent"`
		rootCmd.SetArgs([]string{"search", "nonexistent"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Exit code is 0 and appropriate message shown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}
		if !strings.Contains(output, "No records found") {
			t.Errorf("expected 'No records found' message, output: %s", output)
		}
	})

	t.Run("search requires search term argument", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name"})
		defer cleanup()

		// When: User runs `stash search` without argument
		rootCmd.SetArgs([]string{"search"})
		err := rootCmd.Execute()

		// Then: Command fails with error (cobra returns error for missing arguments)
		if err == nil {
			t.Error("expected error when search term is missing")
		}
	})

	t.Run("search without stash shows error", func(t *testing.T) {
		// Given: No stash directory
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// When: User runs `stash search "test"`
		rootCmd.SetArgs([]string{"search", "test"})
		rootCmd.Execute()

		// Then: Exit code is non-zero
		if ExitCode == 0 {
			t.Error("expected non-zero exit code when no stash exists")
		}
	})

	t.Run("search partial match works", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name", "Description"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney Corporation", "--set", "Description=Media"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Apple Inc", "--set", "Description=Tech"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "dis"` (partial match)
		rootCmd.SetArgs([]string{"search", "dis"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Partial matches are found
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(output, "Disney") {
			t.Errorf("expected Disney Corporation to be shown with partial match, output: %s", output)
		}
	})

	t.Run("search excludes deleted records by default", func(t *testing.T) {
		// Given: Stash with a deleted record
		_, cleanup := setupTestStashWithColumns(t, "companies", "cmp-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Disney"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "DeletedCompany"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Delete the second record
		// First get the list to find the ID
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

		var records []map[string]interface{}
		json.Unmarshal([]byte(output), &records)

		var deleteID string
		for _, rec := range records {
			if rec["Name"] == "DeletedCompany" {
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

		// Capture stdout for search
		r, w, _ = os.Pipe()
		os.Stdout = w

		// When: User runs `stash search "company"`
		rootCmd.SetArgs([]string{"search", "company"})
		rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf = make([]byte, 8192)
		n, _ = r.Read(buf)
		output = string(buf[:n])

		// Then: Deleted record is not shown
		if strings.Contains(output, "DeletedCompany") {
			t.Error("expected deleted record to NOT be shown in search results")
		}
	})
}
