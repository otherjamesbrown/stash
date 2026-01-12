// Package cli provides the command-line interface for stash.
package cli

import (
	"github.com/spf13/cobra"
)

// helpTopicsCmd is a parent command for help topics
var helpTopicsCmd = &cobra.Command{
	Use:   "help-topic",
	Short: "Extended help topics",
	Long:  `Extended help topics for stash. Use 'stash help-topic <topic>' to view.`,
}

var helpJSONCmd = &cobra.Command{
	Use:   "json",
	Short: "JSON output format and schema documentation",
	Long: `JSON Output Format and Schema Documentation

Stash supports JSON output for all commands via the --json flag,
enabling AI agents to parse and process data programmatically.

SYSTEM FIELDS
─────────────
All records include these system fields (prefixed with underscore):

  _id          Unique record identifier (e.g., "inv-ex4j")
  _created_at  ISO 8601 timestamp of creation
  _created_by  Actor who created the record
  _updated_at  ISO 8601 timestamp of last update
  _updated_by  Actor who last updated the record
  _parent_id   Parent record ID (if child record)
  _deleted     true if record is soft-deleted
  _deleted_at  ISO 8601 timestamp of deletion
  _deleted_by  Actor who deleted the record

RECORD JSON FORMAT
──────────────────
Single record (stash show, stash add --json):

  {
    "_id": "inv-ex4j",
    "_created_at": "2026-01-12T10:30:00Z",
    "_created_by": "agent-1",
    "_updated_at": "2026-01-12T10:30:00Z",
    "_updated_by": "agent-1",
    "Name": "Laptop",
    "Price": 999,
    "Category": "electronics"
  }

LIST JSON FORMAT
────────────────
Multiple records (stash list --json, stash query --json):

  [
    {"_id": "inv-ex4j", "Name": "Laptop", "Price": 999},
    {"_id": "inv-7k2m", "Name": "Mouse", "Price": 25}
  ]

Empty result returns: []

QUERY JSON FORMAT
─────────────────
Raw SQL queries return column values as returned by SQLite:

  stash query "SELECT id, Name, Price FROM inventory" --json

  [
    {"id": "inv-ex4j", "Name": "Laptop", "Price": 999},
    {"id": "inv-7k2m", "Name": "Mouse", "Price": 25}
  ]

Note: Query results use column aliases, not system field names.

PARSING WITH JQ
───────────────
Extract IDs:
  stash list --json | jq -r '.[]._id'

Filter by field:
  stash list --json | jq '.[] | select(.Price > 100)'

Count records:
  stash list --json | jq 'length'

Get specific fields:
  stash list --json | jq -r '.[] | "\(._id): \(.Name)"'

PARSING WITH PYTHON
───────────────────
  import subprocess
  import json

  result = subprocess.run(
      ["stash", "list", "--json"],
      capture_output=True, text=True
  )
  records = json.loads(result.stdout)
  for rec in records:
      print(f"{rec['_id']}: {rec['Name']}")

EXIT CODES
──────────
  0  Success
  1  Resource not found (record, stash, column)
  2  Validation error (invalid input)
  3  Conflict (duplicate, constraint violation)
  4  Reference error (invalid parent ID)

ERROR RESPONSES
───────────────
When --json is used and an error occurs, a structured error is returned:

  {
    "error": true,
    "code": "RECORD_NOT_FOUND",
    "message": "Record 'inv-xxxx' not found",
    "details": {"record_id": "inv-xxxx"}
  }

Check exit code to detect errors, then parse the error response if needed.

Related Topics:
  stash help-topic agents    AI agent workflow patterns`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(cmd.Long)
	},
}

var helpAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "AI agent workflows and automation patterns",
	Long: `AI Agent Workflows and Automation Patterns

Stash is designed as an "agent-native" data store, optimized for
AI agents that need to collect, process, and query structured data.

CORE PATTERNS
─────────────

1. Always Use --json for Programmatic Access
   AI agents should always use --json flag for reliable parsing:

   stash list --json | jq -r '.[]._id'
   stash show inv-ex4j --json | jq '.Name'

2. Use SQL Queries for Complex Filtering
   The query command accepts full SQL:

   stash query "SELECT id, Name FROM inventory WHERE Price > 100" --json

BATCH PROCESSING PATTERN
────────────────────────
Process records in a queue-based workflow:

  # Get pending records
  PENDING=$(stash query "SELECT id FROM tasks WHERE status IS NULL" --json)

  # Process each record
  echo "$PENDING" | jq -r '.[].id' | while read id; do
      # Mark as processing
      stash set "$id" status="processing"

      # Do AI agent work here...
      result="completed successfully"

      # Mark as complete
      stash set "$id" status="complete" result="$result"
  done

BULK UPDATE PATTERN
───────────────────
Update multiple records matching a condition:

  # Update all pending to processing
  stash bulk-set --where "status IS NULL" --set status="processing"

  # Update with multiple conditions
  stash bulk-set --where "priority=high" --where "status=pending" \
      --set status="urgent" --set assigned_to="agent-1"

SEARCH PATTERN
──────────────
Find records by text content:

  # Search all text fields
  stash search "keyword" --json

  # Search specific columns
  stash search "keyword" --in Name --in Description --json

MULTI-AGENT COORDINATION
────────────────────────
When multiple agents work on the same stash:

  # Claim records with agent identifier
  stash bulk-set --where "status IS NULL" --where "assigned_to IS NULL" \
      --set status="claimed" --set assigned_to="$AGENT_ID"

  # Process only your claimed records
  stash query "SELECT id FROM tasks WHERE assigned_to='$AGENT_ID'" --json

  # Release uncompleted claims on exit
  stash bulk-set --where "assigned_to='$AGENT_ID'" --where "status=claimed" \
      --set status="" --set assigned_to=""

ACTOR TRACKING
──────────────
Set the actor name for audit trails:

  export STASH_ACTOR="research-agent-1"
  stash add "New Record"  # _created_by will be "research-agent-1"

  # Or per-command:
  stash set inv-ex4j status="done" --actor "cleanup-agent"

ERROR HANDLING
──────────────
Always check exit codes in scripts:

  if ! stash set "$id" field="value" 2>/dev/null; then
      echo "Failed to update $id" >&2
      stash set "$ERROR_LOG_ID" error="update failed for $id"
  fi

Exit codes:
  0 - Success
  1 - Resource not found
  2 - Validation error
  3 - Conflict
  4 - Reference error

PERFORMANCE TIPS
────────────────
1. Use specific WHERE clauses to limit results
2. Add LIMIT to queries when processing in batches
3. Use --quiet flag to suppress non-essential output
4. Use bulk-set for updating multiple records (faster than loop)

CSV EXPORT FOR REPORTS
──────────────────────
Export data for external processing:

  # Full CSV with headers
  stash query "SELECT * FROM inventory" --csv

  # Specific columns, no headers (for piping)
  stash query "SELECT id, Name FROM inventory" --csv --no-headers

  # Select columns from result
  stash query "SELECT * FROM inventory" --csv --columns "Name,Price"

Related Topics:
  stash help-topic json      JSON schema and parsing examples`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(cmd.Long)
	},
}

func init() {
	helpTopicsCmd.AddCommand(helpJSONCmd)
	helpTopicsCmd.AddCommand(helpAgentsCmd)
	rootCmd.AddCommand(helpTopicsCmd)
}
