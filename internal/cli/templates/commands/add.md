# Add Stash Record

Add a new record to the current stash.

## Usage

Run `./stash add <value>` with optional field settings.

## Examples

```bash
# Add a simple record
./stash add "Laptop"

# Add with additional fields
./stash add "Laptop" --set Price=999 --set Category="electronics"

# Add as child of another record
./stash add "Charger" --parent inv-ex4j
```

## Instructions

Run `./stash add` with the provided value and any field settings. The value goes to the primary column.

$ARGUMENTS
