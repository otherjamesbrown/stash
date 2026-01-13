package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/cli/templates"
	"github.com/user/stash/internal/context"
)

var forceInstall bool

var initClaudeCmd = &cobra.Command{
	Use:   "init-claude",
	Short: "Install Claude Code integration",
	Long: `Install Claude Code slash commands and update settings.

This command sets up Claude Code integration by:
  1. Creating .claude/commands/stash/ with slash command files
  2. Adding stash:* to .claude/settings.json allowedBashCommands
  3. Appending onboarding snippet to CLAUDE.md

Examples:
  stash init-claude           # Install Claude integration
  stash init-claude --force   # Overwrite existing files`,
	Args: cobra.NoArgs,
	RunE: runInitClaude,
}

func init() {
	initClaudeCmd.Flags().BoolVar(&forceInstall, "force", false, "Overwrite existing files")
	rootCmd.AddCommand(initClaudeCmd)
}

// initClaudeResult holds the result of the init-claude command for JSON output.
type initClaudeResult struct {
	InstalledFiles   []string `json:"installed_files"`
	SettingsUpdated  bool     `json:"settings_updated"`
	ClaudeMDUpdated  bool     `json:"claude_md_updated"`
	SettingsCreated  bool     `json:"settings_created"`
	ClaudeMDCreated  bool     `json:"claude_md_created"`
}

func runInitClaude(cmd *cobra.Command, args []string) error {
	result := initClaudeResult{
		InstalledFiles: []string{},
	}

	// Check if already installed (unless --force)
	commandsDir := filepath.Join(".claude", "commands", "stash")
	if !forceInstall {
		if _, err := os.Stat(commandsDir); err == nil {
			// Directory exists, check for files
			entries, _ := os.ReadDir(commandsDir)
			if len(entries) > 0 {
				fmt.Fprintln(os.Stderr, "Error: Claude integration already installed")
				fmt.Fprintln(os.Stderr, "Use --force to overwrite existing files")
				Exit(1)
				return nil
			}
		}
	}

	// Create .claude/commands/stash/ directory
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	// Copy embedded templates
	templateFiles := []string{"list.md", "add.md", "show.md", "set.md", "rm.md", "query.md"}
	for _, filename := range templateFiles {
		content, err := templates.Commands.ReadFile(filepath.Join("commands", filename))
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", filename, err)
		}

		destPath := filepath.Join(commandsDir, filename)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}
		result.InstalledFiles = append(result.InstalledFiles, destPath)
	}

	// Update .claude/settings.json
	settingsUpdated, settingsCreated, err := updateClaudeSettings()
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	result.SettingsUpdated = settingsUpdated
	result.SettingsCreated = settingsCreated

	// Append to CLAUDE.md
	claudeMDUpdated, claudeMDCreated, err := updateClaudeMD()
	if err != nil {
		return fmt.Errorf("failed to update CLAUDE.md: %w", err)
	}
	result.ClaudeMDUpdated = claudeMDUpdated
	result.ClaudeMDCreated = claudeMDCreated

	// Output result
	if GetJSONOutput() {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Println("Claude Code integration installed successfully!")
		fmt.Println()
		fmt.Println("Slash commands installed:")
		for _, f := range result.InstalledFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
		if result.SettingsCreated {
			fmt.Println("Created .claude/settings.json with stash:* permission")
		} else if result.SettingsUpdated {
			fmt.Println("Updated .claude/settings.json with stash:* permission")
		}
		if result.ClaudeMDCreated {
			fmt.Println("Created CLAUDE.md with stash documentation")
		} else if result.ClaudeMDUpdated {
			fmt.Println("Appended stash documentation to CLAUDE.md")
		}
		fmt.Println()
		fmt.Println("Available slash commands:")
		fmt.Println("  /stash:list   - List records")
		fmt.Println("  /stash:add    - Add a record")
		fmt.Println("  /stash:show   - Show record details")
		fmt.Println("  /stash:set    - Update a record")
		fmt.Println("  /stash:rm     - Delete a record")
		fmt.Println("  /stash:query  - Run a SQL query")
	}

	return nil
}

// updateClaudeSettings updates .claude/settings.json to include stash:* in allowedBashCommands.
func updateClaudeSettings() (updated bool, created bool, err error) {
	settingsPath := filepath.Join(".claude", "settings.json")

	// Ensure .claude directory exists
	if err := os.MkdirAll(".claude", 0755); err != nil {
		return false, false, err
	}

	// Read existing settings or create new
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new settings
			settings = map[string]interface{}{
				"allowedBashCommands": []interface{}{"stash:*"},
			}
			created = true
		} else {
			return false, false, err
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, false, fmt.Errorf("failed to parse settings.json: %w", err)
		}
	}

	// Check/update allowedBashCommands
	if !created {
		commands, ok := settings["allowedBashCommands"].([]interface{})
		if !ok {
			commands = []interface{}{}
		}

		// Check if stash:* already exists
		hasStash := false
		for _, cmd := range commands {
			if cmdStr, ok := cmd.(string); ok && cmdStr == "stash:*" {
				hasStash = true
				break
			}
		}

		if !hasStash {
			commands = append(commands, "stash:*")
			settings["allowedBashCommands"] = commands
			updated = true
		}
	}

	// Write settings if changed
	if created || updated {
		newData, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return false, false, err
		}
		if err := os.WriteFile(settingsPath, newData, 0644); err != nil {
			return false, false, err
		}
	}

	return updated, created, nil
}

// updateClaudeMD appends the stash onboarding snippet to CLAUDE.md.
func updateClaudeMD() (updated bool, created bool, err error) {
	claudeMDPath := "CLAUDE.md"

	// Resolve context for the snippet
	ctx, _ := context.Resolve(GetActorName(), "")

	snippet := generateOnboardingSnippet(ctx)

	// Check if file exists
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file
			if err := os.WriteFile(claudeMDPath, []byte(snippet), 0644); err != nil {
				return false, false, err
			}
			return false, true, nil
		}
		return false, false, err
	}

	// Check if snippet already exists (look for marker)
	marker := "## Stash - Structured Data Store"
	if contains(string(data), marker) {
		// Already has stash section
		return false, false, nil
	}

	// Append to existing file
	f, err := os.OpenFile(claudeMDPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, false, err
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + snippet); err != nil {
		return false, false, err
	}

	return true, false, nil
}

// generateOnboardingSnippet generates the CLAUDE.md snippet for stash.
func generateOnboardingSnippet(ctx *context.Context) string {
	actorStr := "unknown"
	branchStr := "unknown"
	if ctx != nil {
		actorStr = ctx.Actor
		branchStr = ctx.Branch
	}

	return `## Stash - Structured Data Store

Stash is a structured data store for collecting and querying data.

### Quick Reference

` + "```" + `bash
# 1. Initialize stash
stash init mydata --prefix dat-

# 2. Define columns (names: letters, numbers, underscores only - no hyphens)
stash column add company_name
stash column add status
stash column add notes

# 3. Add records
stash add "Acme Corp" --set status=active

# View all stashes and status
stash info

# List records
stash list
stash list --json

# Show a specific record
stash show <id>

# Update a record
stash set <id> field=value

# Delete a record (soft-delete)
stash rm <id>

# Query records
stash query "SELECT * FROM stash_name WHERE field = 'value'"
` + "```" + `

### Current Context
- Actor: ` + actorStr + `
- Branch: ` + branchStr + `

### Slash Commands

The following slash commands are available:
- ` + "`/stash:list`" + ` - List records with optional filters
- ` + "`/stash:add`" + ` - Add a new record
- ` + "`/stash:show`" + ` - Show record details
- ` + "`/stash:set`" + ` - Update record fields
- ` + "`/stash:rm`" + ` - Delete a record
- ` + "`/stash:query`" + ` - Run SQL queries
`
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
