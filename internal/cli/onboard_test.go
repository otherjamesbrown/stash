package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestUC_ST_004_GenerateOnboarding tests UC-ST-004: Generate Onboarding Snippet
func TestUC_ST_004_GenerateOnboarding(t *testing.T) {
	t.Run("AC-01: output markdown snippet", func(t *testing.T) {
		// Given: Stash "inventory" exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create stash
		rootCmd.SetArgs([]string{"init", "inventory", "--prefix", "inv-"})
		rootCmd.Execute()

		// Capture output
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(&buf)
		buf.Reset()

		// When: User runs `stash onboard`
		rootCmd.SetArgs([]string{"onboard"})
		err := rootCmd.Execute()

		// Then: Exit code is 0
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Output goes to stdout via fmt.Print, not buf
		// We verify the command runs without error
	})

	t.Run("output contains quick reference commands", func(t *testing.T) {
		// Given: Any environment
		_, cleanup := setupTestEnv(t)
		defer cleanup()

		// Capture stdout by redirecting
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash onboard`
		rootCmd.SetArgs([]string{"onboard"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Then: Output is valid markdown
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Then: Contains essential quick reference commands
		expectedCommands := []string{
			"stash init",
			"stash column add",
			"stash add",
			"stash list",
			"stash show",
			"stash set",
			"stash rm",
		}

		for _, cmd := range expectedCommands {
			if !strings.Contains(output, cmd) {
				t.Errorf("expected output to contain command %q", cmd)
			}
		}

		// Then: Points to stash prime for full workflow
		if !strings.Contains(output, "stash prime") {
			t.Error("expected output to reference 'stash prime' for full workflow details")
		}

		// Then: Contains markdown formatting
		if !strings.Contains(output, "##") {
			t.Error("expected output to contain markdown headers")
		}
		if !strings.Contains(output, "```") {
			t.Error("expected output to contain code blocks")
		}
	})
}

// TestUC_ST_004_GenerateOnboarding_MustNot tests anti-requirements
func TestUC_ST_004_GenerateOnboarding_MustNot(t *testing.T) {
	t.Run("must not modify any files", func(t *testing.T) {
		// Given: A directory with existing files
		tempDir, cleanup := setupTestEnv(t)
		defer cleanup()

		// Create a CLAUDE.md file
		testFile := tempDir + "/CLAUDE.md"
		os.WriteFile(testFile, []byte("# Original Content\n"), 0644)

		// Read original content
		origContent, _ := os.ReadFile(testFile)

		// When: User runs `stash onboard`
		rootCmd.SetArgs([]string{"onboard"})
		rootCmd.Execute()

		// Then: CLAUDE.md should be unchanged
		newContent, _ := os.ReadFile(testFile)
		if string(origContent) != string(newContent) {
			t.Error("expected CLAUDE.md to remain unchanged")
		}
	})
}
