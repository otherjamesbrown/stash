# List Stash Records

List records in the current stash.

## Usage

Run `./stash list` with optional filters.

## Examples

```bash
# List root records
./stash list

# List all records including children
./stash list --all

# List children of a specific parent
./stash list --parent <id>

# Search across all fields
./stash list --search "term"

# Filter by field value
./stash list --where "Price>100"
```

## Instructions

Run `./stash list` and display the results. If the user provided arguments after `/stash:list`, pass them to the command.

$ARGUMENTS
