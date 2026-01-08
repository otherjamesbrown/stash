package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestUC_ST_005_GenerateContext tests UC-ST-005: Generate Context for Agent
func TestUC_ST_005_GenerateContext(t *testing.T) {
	t.Run("AC-01: output full context", func(t *testing.T) {
		// Given: Stash "inventory" exists with columns and records
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash prime`
		rootCmd.SetArgs([]string{"prime"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Output includes actor and branch
		if !strings.Contains(output, "Actor") {
			t.Error("expected output to contain actor information")
		}
		if !strings.Contains(output, "Branch") {
			t.Error("expected output to contain branch information")
		}

		// Then: Output includes stash name
		if !strings.Contains(output, "inventory") {
			t.Error("expected output to contain stash name 'inventory'")
		}

		// Then: Output includes column list (header at least)
		if !strings.Contains(output, "Columns") {
			t.Error("expected output to contain Columns section")
		}

		// Then: Output includes statistics
		if !strings.Contains(output, "Statistics") {
			t.Error("expected output to contain Statistics section")
		}

		// Then: Output includes recent changes
		if !strings.Contains(output, "Recent Changes") {
			t.Error("expected output to contain Recent Changes section")
		}
	})

	t.Run("AC-02: filter to specific stash", func(t *testing.T) {
		// Given: Multiple stashes exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create multiple stashes
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		rootCmd.SetArgs([]string{"init", "contacts", "--prefix", "ct-"})
		rootCmd.Execute()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash prime --stash inventory`
		rootCmd.SetArgs([]string{"prime", "--stash", "inventory"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Output only includes inventory stash context
		if !strings.Contains(output, "inventory") {
			t.Error("expected output to contain 'inventory'")
		}

		// Then: Output should not include contacts stash
		// (Check for contacts as a section header - it would appear as "## contacts")
		if strings.Contains(output, "## contacts") {
			t.Error("expected output NOT to contain contacts stash section")
		}
	})

	t.Run("shows no stashes message when empty", func(t *testing.T) {
		// Given: No stashes exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash prime`
		rootCmd.SetArgs([]string{"prime"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Then: No error
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Output mentions no stashes
		if !strings.Contains(output, "No stashes") && !strings.Contains(output, "no stashes") {
			t.Error("expected output to mention no stashes found")
		}
	})

	t.Run("shows markdown format", func(t *testing.T) {
		// Given: Stash exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "test", "--prefix", "ts-"})
		rootCmd.Execute()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash prime`
		rootCmd.SetArgs([]string{"prime"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Then: Output is markdown formatted
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Check for markdown headers
		if !strings.Contains(output, "#") {
			t.Error("expected output to contain markdown headers")
		}

		// Check for bold formatting
		if !strings.Contains(output, "**") {
			t.Error("expected output to contain bold formatting")
		}
	})
}

// TestUC_ST_005_GenerateContext_MustNot tests anti-requirements
func TestUC_ST_005_GenerateContext_MustNot(t *testing.T) {
	t.Run("must not modify any files", func(t *testing.T) {
		// Given: Stash exists
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Get file list before prime
		filesBefore, _ := os.ReadDir(tempDir)
		stashDirBefore, _ := os.ReadDir(tempDir + "/.stash/inventory")

		// When: User runs `stash prime`
		rootCmd.SetArgs([]string{"prime"})
		rootCmd.Execute()

		// Then: No new files should be created
		filesAfter, _ := os.ReadDir(tempDir)
		stashDirAfter, _ := os.ReadDir(tempDir + "/.stash/inventory")

		if len(filesBefore) != len(filesAfter) {
			t.Error("expected no new files in temp directory")
		}

		if len(stashDirBefore) != len(stashDirAfter) {
			t.Error("expected no new files in stash directory")
		}
	})
}
