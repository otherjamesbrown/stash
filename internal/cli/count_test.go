package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCountCommand(t *testing.T) {
	t.Run("count all records", func(t *testing.T) {
		// Given: Stash with multiple records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Keyboard"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count`
		rootCmd.SetArgs([]string{"count"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is "3"
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output != "3" {
			t.Errorf("expected count 3, got %q", output)
		}
	})

	t.Run("count with WHERE clause", func(t *testing.T) {
		// Given: Stash with records having different categories
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Category"})
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

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count --where "Category=electronics"`
		rootCmd.SetArgs([]string{"count", "--where", "Category=electronics"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is "2"
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output != "2" {
			t.Errorf("expected count 2, got %q", output)
		}
	})

	t.Run("count with IS NULL", func(t *testing.T) {
		// Given: Stash with some records having NULL values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Keyboard"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count --where "Notes IS NULL"`
		rootCmd.SetArgs([]string{"count", "--where", "Notes IS NULL"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is "2" (Mouse and Keyboard have NULL Notes)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output != "2" {
			t.Errorf("expected count 2, got %q", output)
		}
	})

	t.Run("count with IS NOT EMPTY", func(t *testing.T) {
		// Given: Stash with NULL, empty, and populated values
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name", "Notes"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop", "--set", "Notes=Has charger"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Mouse"}) // NULL
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Keyboard", "--set", "Notes="}) // empty string
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count --where "Notes IS NOT EMPTY"`
		rootCmd.SetArgs([]string{"count", "--where", "Notes IS NOT EMPTY"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is "1" (only Laptop has non-empty Notes)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output != "1" {
			t.Errorf("expected count 1, got %q", output)
		}
	})

	t.Run("count with JSON output", func(t *testing.T) {
		// Given: Stash with records
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		rootCmd.SetArgs([]string{"add", "Laptop"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		rootCmd.SetArgs([]string{"add", "Mouse"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count --json`
		rootCmd.SetArgs([]string{"count", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is valid JSON with count
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var result map[string]int
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("expected valid JSON, got error: %v, output: %s", err, output)
		}
		if result["count"] != 2 {
			t.Errorf("expected count 2, got %d", result["count"])
		}
	})

	t.Run("count empty stash", func(t *testing.T) {
		// Given: Empty stash
		_, cleanup := setupTestStashWithColumns(t, "inventory", "inv-", []string{"Name"})
		defer cleanup()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash count`
		rootCmd.SetArgs([]string{"count"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		output := strings.TrimSpace(string(buf[:n]))

		// Then: Output is "0"
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output != "0" {
			t.Errorf("expected count 0, got %q", output)
		}
	})
}
