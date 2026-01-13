package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Generate CLAUDE.md onboarding snippet",
	Long: `Generate a markdown snippet for CLAUDE.md with quick reference commands.

This outputs a ready-to-paste section that can be added to your CLAUDE.md
file to help AI agents understand how to use stash.

Example:
  stash onboard >> CLAUDE.md`,
	Args: cobra.NoArgs,
	RunE: runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

func runOnboard(cmd *cobra.Command, args []string) error {
	snippet := `## Stash - Structured Data Store

Stash is a structured data store for AI agents.
Run ` + "`stash prime`" + ` for workflow context.

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

# Working with records
stash list                           # List all
stash show <id>                      # Show details
stash set <id> field=value           # Update
stash rm <id>                        # Delete

# Query and export
stash list --where "field=value"
stash export records.jsonl
` + "```" + `

For full workflow details: ` + "`stash prime`" + `
`

	fmt.Print(snippet)
	return nil
}
