package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUC_ST_006_InitClaude tests UC-ST-006: Initialize Claude Integration
func TestUC_ST_006_InitClaude(t *testing.T) {
	t.Run("AC-01: create slash command files", func(t *testing.T) {
		// Given: .claude/commands/stash/ does not exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// When: User runs `stash init-claude`
		rootCmd.SetArgs([]string{"init-claude"})
		err := rootCmd.Execute()

		// Then: Directory .claude/commands/stash/ is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", ExitCode)
		}

		commandsDir := filepath.Join(".claude", "commands", "stash")
		if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
			t.Error("expected .claude/commands/stash/ directory to exist")
		}

		// Then: Slash command files are created
		expectedFiles := []string{"list.md", "add.md", "show.md", "set.md", "rm.md", "query.md"}
		for _, filename := range expectedFiles {
			filePath := filepath.Join(commandsDir, filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("expected %s to exist", filePath)
			}
		}
	})

	t.Run("AC-02: update settings.json", func(t *testing.T) {
		// Given: .claude/settings.json exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create existing settings.json
		os.MkdirAll(".claude", 0755)
		existingSettings := map[string]interface{}{
			"allowedBashCommands": []interface{}{"git:*", "go:*"},
			"otherSetting":        "value",
		}
		data, _ := json.MarshalIndent(existingSettings, "", "  ")
		os.WriteFile(filepath.Join(".claude", "settings.json"), data, 0644)

		// When: User runs `stash init-claude`
		rootCmd.SetArgs([]string{"init-claude"})
		err := rootCmd.Execute()

		// Then: stash:* is added to allowedBashCommands array
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		settingsData, err := os.ReadFile(filepath.Join(".claude", "settings.json"))
		if err != nil {
			t.Fatalf("failed to read settings.json: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(settingsData, &settings); err != nil {
			t.Fatalf("failed to parse settings.json: %v", err)
		}

		// Then: Existing settings are preserved
		if settings["otherSetting"] != "value" {
			t.Error("expected otherSetting to be preserved")
		}

		commands, ok := settings["allowedBashCommands"].([]interface{})
		if !ok {
			t.Fatal("expected allowedBashCommands to be an array")
		}

		hasStash := false
		hasGit := false
		for _, cmd := range commands {
			if cmdStr, ok := cmd.(string); ok {
				if cmdStr == "stash:*" {
					hasStash = true
				}
				if cmdStr == "git:*" {
					hasGit = true
				}
			}
		}
		if !hasStash {
			t.Error("expected stash:* to be in allowedBashCommands")
		}
		if !hasGit {
			t.Error("expected git:* to be preserved in allowedBashCommands")
		}
	})

	t.Run("AC-03: create settings.json if missing", func(t *testing.T) {
		// Given: .claude/settings.json does not exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// When: User runs `stash init-claude`
		rootCmd.SetArgs([]string{"init-claude"})
		err := rootCmd.Execute()

		// Then: .claude/settings.json is created
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		settingsData, err := os.ReadFile(filepath.Join(".claude", "settings.json"))
		if err != nil {
			t.Fatalf("expected settings.json to be created: %v", err)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(settingsData, &settings); err != nil {
			t.Fatalf("failed to parse settings.json: %v", err)
		}

		// Then: Contains allowedBashCommands with stash:*
		commands, ok := settings["allowedBashCommands"].([]interface{})
		if !ok {
			t.Fatal("expected allowedBashCommands to be an array")
		}

		hasStash := false
		for _, cmd := range commands {
			if cmdStr, ok := cmd.(string); ok && cmdStr == "stash:*" {
				hasStash = true
				break
			}
		}
		if !hasStash {
			t.Error("expected stash:* to be in allowedBashCommands")
		}
	})

	t.Run("AC-04: append to CLAUDE.md", func(t *testing.T) {
		// Given: CLAUDE.md exists
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		existingContent := "# My Project\n\nThis is my project.\n"
		os.WriteFile("CLAUDE.md", []byte(existingContent), 0644)

		// When: User runs `stash init-claude`
		rootCmd.SetArgs([]string{"init-claude"})
		err := rootCmd.Execute()

		// Then: Onboarding snippet is appended
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		content, err := os.ReadFile("CLAUDE.md")
		if err != nil {
			t.Fatalf("failed to read CLAUDE.md: %v", err)
		}

		contentStr := string(content)

		// Then: Existing content is preserved
		if !strings.Contains(contentStr, "# My Project") {
			t.Error("expected existing content to be preserved")
		}

		// Then: Stash section is added
		if !strings.Contains(contentStr, "## Stash - Structured Data Store") {
			t.Error("expected stash section to be appended")
		}
	})

	t.Run("AC-05: create CLAUDE.md if missing", func(t *testing.T) {
		// Given: CLAUDE.md does not exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// When: User runs `stash init-claude`
		rootCmd.SetArgs([]string{"init-claude"})
		err := rootCmd.Execute()

		// Then: CLAUDE.md is created with onboarding snippet
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		content, err := os.ReadFile("CLAUDE.md")
		if err != nil {
			t.Fatalf("expected CLAUDE.md to be created: %v", err)
		}

		if !strings.Contains(string(content), "## Stash - Structured Data Store") {
			t.Error("expected stash section in CLAUDE.md")
		}
	})

	t.Run("AC-06: fail if already installed", func(t *testing.T) {
		// Given: Slash commands already exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Install first
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// When: User runs `stash init-claude` again
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()

		// Then: Command fails with exit code 1
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}
	})

	t.Run("AC-07: overwrite with --force", func(t *testing.T) {
		// Given: Slash commands already exist
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Install first
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Modify a file to verify it gets overwritten
		testFile := filepath.Join(".claude", "commands", "stash", "list.md")
		os.WriteFile(testFile, []byte("modified content"), 0644)

		// When: User runs `stash init-claude --force`
		rootCmd.SetArgs([]string{"init-claude", "--force"})
		err := rootCmd.Execute()

		// Then: Files are overwritten
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", ExitCode)
		}

		// Verify file was overwritten
		content, _ := os.ReadFile(testFile)
		if string(content) == "modified content" {
			t.Error("expected list.md to be overwritten")
		}
	})

	t.Run("AC-08: JSON output format", func(t *testing.T) {
		// Given: Any state
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// When: User runs `stash init-claude --json`
		rootCmd.SetArgs([]string{"init-claude", "--json"})
		err := rootCmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Read captured output
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Then: Output is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
		}

		// Then: Contains installed_files array
		if _, ok := result["installed_files"]; !ok {
			t.Error("expected installed_files in JSON output")
		}

		// Then: Contains settings_updated boolean
		if _, ok := result["settings_updated"]; !ok {
			t.Error("expected settings_updated in JSON output")
		}

		// Then: Contains claude_md_updated boolean
		if _, ok := result["claude_md_updated"]; !ok {
			t.Error("expected claude_md_updated in JSON output")
		}
	})
}

// TestUC_ST_006_InitClaude_MustNot tests the must_not constraints
func TestUC_ST_006_InitClaude_MustNot(t *testing.T) {
	t.Run("must not overwrite existing files without --force", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Install first
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()
		ExitCode = 0
		resetFlags()

		// Modify a file
		testFile := filepath.Join(".claude", "commands", "stash", "list.md")
		originalContent := "modified content"
		os.WriteFile(testFile, []byte(originalContent), 0644)

		// Try to install again without --force
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()

		// Should fail
		if ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", ExitCode)
		}

		// File should not be modified
		content, _ := os.ReadFile(testFile)
		if string(content) != originalContent {
			t.Error("expected file to not be overwritten without --force")
		}
	})

	t.Run("must not modify unrelated settings", func(t *testing.T) {
		_, cleanup := setupTestEnv(t)
		defer cleanup()
		resetFlags()

		// Create settings with other content
		os.MkdirAll(".claude", 0755)
		existingSettings := map[string]interface{}{
			"allowedBashCommands": []interface{}{"git:*"},
			"customSetting":       "custom value",
			"nestedSetting": map[string]interface{}{
				"key": "value",
			},
		}
		data, _ := json.MarshalIndent(existingSettings, "", "  ")
		os.WriteFile(filepath.Join(".claude", "settings.json"), data, 0644)

		// Run init-claude
		rootCmd.SetArgs([]string{"init-claude"})
		rootCmd.Execute()

		// Read updated settings
		settingsData, _ := os.ReadFile(filepath.Join(".claude", "settings.json"))
		var settings map[string]interface{}
		json.Unmarshal(settingsData, &settings)

		// Verify unrelated settings are preserved
		if settings["customSetting"] != "custom value" {
			t.Error("expected customSetting to be preserved")
		}

		nested, ok := settings["nestedSetting"].(map[string]interface{})
		if !ok || nested["key"] != "value" {
			t.Error("expected nestedSetting to be preserved")
		}
	})
}
