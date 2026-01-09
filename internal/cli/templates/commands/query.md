# Query Stash

Run a SQL SELECT query against the stash cache.

## Usage

Run `./stash query "<sql>"` to execute a query.

## Examples

```bash
# Select with filter
./stash query "SELECT Name, Price FROM inventory WHERE Price > 100"

# Aggregate query
./stash query "SELECT Category, COUNT(*) FROM inventory GROUP BY Category"

# Recent records
./stash query "SELECT * FROM inventory ORDER BY updated_at DESC LIMIT 10"
```

## Instructions

Run `./stash query` with a SQL SELECT statement. The table name is the stash name.

$ARGUMENTS
