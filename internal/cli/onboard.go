package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
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
	// Resolve context
	ctx, _ := context.Resolve(GetActorName(), "")

	snippet := `## Stash - Structured Data Store

Stash is a structured data store for collecting and querying data.

### Quick Reference

` + "```" + `bash
# View all stashes and status
stash info

# Add a record (primary value goes to first column)
stash add "value" --set field=value

# List records
stash list
stash list --json

# Get a specific record
stash get <id>

# Update a record
stash set <id> field=value

# Delete a record (soft-delete)
stash rm <id>

# Query records
stash query "field = 'value'"
` + "```" + `

### Current Context
- Actor: ` + ctx.Actor + `
- Branch: ` + ctx.Branch + `

### Column Operations

` + "```" + `bash
# Add a new column
stash col add <name> --desc "Description"

# List columns
stash col list
` + "```" + `

### JSON Output

All commands support ` + "`--json`" + ` for structured output:

` + "```" + `bash
stash list --json | jq '.records[].id'
stash info --json
` + "```" + `
`

	fmt.Print(snippet)
	return nil
}
