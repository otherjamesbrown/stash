package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var humanCmd = &cobra.Command{
	Use:   "human",
	Short: "Show essential commands for human users",
	Long: `Display a curated list of essential stash commands for human users.

For the full command list (30+ commands), use: stash --help
For AI agent context, use: stash prime`,
	Args: cobra.NoArgs,
	Run:  runHuman,
}

func init() {
	rootCmd.AddCommand(humanCmd)
}

func runHuman(cmd *cobra.Command, args []string) {
	fmt.Print(`stash - Essential Commands for Humans
For all commands: stash --help

Setup:
  init <name>            Create a new stash with prefix
  column add <names>     Define columns (required before adding records)
  info                   Show stash status and statistics

Working With Records:
  add <value>            Add a record (value goes to first column)
  add <value> --set k=v  Add record with additional fields
  list                   List all records
  show <id>              Show record details
  set <id> key=value     Update a record field
  rm <id>                Soft-delete a record

Querying:
  list --where "k=v"     Filter records
  query "SQL"            Run raw SQL query
  export records.jsonl   Export to file

Column Management:
  column list            Show all columns with stats
  column add <name>      Add a new column
  column describe <n> d  Set column description

Files & Attachments:
  attach <id> <file>     Attach file to record
  files <id>             List attachments
  detach <id> <file>     Remove attachment

Quick Examples:
  # Create a stash and add records
  stash init inventory --prefix inv-
  stash column add Name Price Category
  stash add "Laptop" --set Price=999 --set Category=electronics

  # Query and export
  stash list --where "Category=electronics"
  stash export products.jsonl

  # Get AI context
  stash prime
`)
}
