# Delete Stash Record

Soft-delete a record (can be restored with `stash restore`).

## Usage

Run `./stash rm <id>` to delete a record.

## Examples

```bash
# Delete with confirmation
./stash rm inv-ex4j

# Skip confirmation
./stash rm inv-ex4j --yes

# Delete parent and all children
./stash rm inv-ex4j --cascade
```

## Instructions

Run `./stash rm` with the record ID. Use --yes to skip confirmation.

$ARGUMENTS
