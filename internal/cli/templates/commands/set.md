# Update Stash Record

Update fields on an existing record.

## Usage

Run `./stash set <id> <field>=<value>` to update fields.

## Examples

```bash
# Update single field
./stash set inv-ex4j Price=1299

# Update multiple fields
./stash set inv-ex4j Price=1299 Stock=50

# Clear a field
./stash set inv-ex4j Notes=""
```

## Instructions

Run `./stash set` with the record ID and field=value pairs to update.

$ARGUMENTS
