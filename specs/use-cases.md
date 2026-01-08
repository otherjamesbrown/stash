# Stash: Functional Use Cases

This document provides functional use cases organized by command area. Each use case demonstrates real workflows and serves as the basis for acceptance testing.

> **Goal**: 100% of commands are tested through these use cases.

---

## Command Coverage Matrix

| Area | Commands | Use Cases |
|------|----------|-----------|
| **Stash Management** | init, drop, info, onboard, prime | UC-ST-001 to UC-ST-005 |
| **Column Management** | column add, list, describe, rename, drop | UC-COL-001 to UC-COL-005 |
| **Record Operations** | add, set, show, file, delete, restore, purge | UC-REC-001 to UC-REC-007 |
| **Querying** | list, children, query, history | UC-QRY-001 to UC-QRY-004 |
| **Import/Export** | import, export | UC-IMP-001 to UC-IMP-002 |
| **Maintenance** | sync, doctor, repair | UC-SYN-001 to UC-SYN-003 |
| **Daemon** | daemon start/stop/restart/status/logs | UC-DMN-001 to UC-DMN-005 |

---

## 1. Stash Management

### UC-ST-001: Initialize a New Stash

**Goal**: Create a new stash to store structured data.

```bash
# Basic initialization
stash init inventory --prefix inv-

# Expected output:
# Created stash 'inventory' with prefix 'inv-'
# Location: .stash/inventory/
# Actor: alice
# Branch: main
# Daemon started (PID 12345)
```

**Verification**:
```bash
ls -la .stash/inventory/
# config.json
# records.jsonl
# files/

cat .stash/inventory/config.json | jq .
# {
#   "name": "inventory",
#   "prefix": "inv-",
#   "created": "2025-01-08T10:00:00Z",
#   "created_by": "alice",
#   "columns": []
# }
```

**Variants**:
```bash
# Without daemon
stash init test --prefix ts- --no-daemon

# With custom actor
stash init research --prefix re- --actor "research-bot"
```

**Error Cases**:
```bash
# Duplicate name
stash init inventory --prefix inv-
# Error: stash 'inventory' already exists

# Invalid prefix (too short)
stash init test --prefix a-
# Error: prefix must be 3-5 characters (2-4 letters + dash)

# Invalid prefix (too long)
stash init test --prefix toolong-
# Error: prefix must be 3-5 characters (2-4 letters + dash)

# Invalid prefix (no dash)
stash init test --prefix inv
# Error: prefix must end with dash
```

---

### UC-ST-002: Drop a Stash

**Goal**: Permanently delete a stash and all its data.

```bash
# With confirmation prompt
stash drop old-project
# This will permanently delete stash 'old-project' and all its data.
# Are you sure? [y/N] y
# Dropped stash 'old-project'

# Skip confirmation
stash drop old-project --yes
# Dropped stash 'old-project'
```

**Verification**:
```bash
ls .stash/
# (old-project directory should be gone)

stash info
# (old-project should not be listed)
```

**Error Cases**:
```bash
# Non-existent stash
stash drop fake --yes
# Error: stash 'fake' not found
```

---

### UC-ST-003: View Stash Information

**Goal**: See status of all stashes and current context.

```bash
stash info
```

**Expected Output**:
```
Stashes:
  inventory (inv-)  100 records (3 deleted), 47 files, synced
  contacts (ct-)     25 records (0 deleted),  0 files, synced

Actor: alice
Branch: feature-xyz
Daemon: running (PID 12345), last sync 2s ago
Cache: .stash/cache.db (2.1 MB)
```

**JSON Output**:
```bash
stash info --json
```
```json
{
  "stashes": [
    {
      "name": "inventory",
      "prefix": "inv-",
      "records": 100,
      "deleted": 3,
      "files": 47,
      "synced": true
    }
  ],
  "context": {
    "actor": "alice",
    "branch": "feature-xyz"
  },
  "daemon": {
    "running": true,
    "pid": 12345,
    "last_sync": "2s ago"
  }
}
```

---

### UC-ST-004: Generate Onboarding Snippet

**Goal**: Get CLAUDE.md snippet for agent integration.

```bash
stash onboard
```

**Expected Output**:
```markdown
## Data Management with Stash

This project uses **stash** for structured data storage.
Run `stash prime` for current context.

**Quick reference:**
- `stash list --json` - List all records (structured)
- `stash add "value"` - Create record
- `stash set <id> <col> <val>` - Update field
- `stash file <id> <col> --content "..."` - Attach markdown
- `stash list --where "col = value"` - Filter records
- `stash column list` - See available columns
- `stash delete <id>` - Soft-delete a record
- `stash purge --before 30d` - Permanently remove old deleted records

**Active stashes:** Run `stash info` to see current data.
```

---

### UC-ST-005: Generate Context for Agent

**Goal**: Output current stash context for agent injection.

```bash
stash prime
```

**Expected Output**:
```markdown
# Stash Context

Actor: alice
Branch: feature-xyz

## Active Stashes

### inventory (prefix: inv-)
Columns:
  - Name: Item display name
  - Category: Product category
  - Price: Price in USD

Records: 100 total (3 soft-deleted)
  - 47 in Category = 'electronics'
  - 32 with Price set

Recent changes (last 24h):
  - 5 created by alice
  - 2 updated by bob

## Quick Commands
- Iterate: `stash list --json | jq -c '.[]'`
- Filter: `stash list --where "Category = 'electronics'" --json`
- Update: `stash set <id> <column> <value>`
```

**Specific Stash**:
```bash
stash prime --stash inventory
```

---

## 2. Column Management

### UC-COL-001: Add Columns

**Goal**: Define schema by adding columns to a stash.

```bash
# Add single column
stash column add Name
# Added column 'Name'

# Add multiple columns
stash column add Price Category Stock
# Added columns: Price, Category, Stock

# Add with description
stash column add Notes --desc "Additional notes or comments"
# Added column 'Notes' with description
```

**Verification**:
```bash
stash column list
# Column    Description                    Populated  Empty
# ───────────────────────────────────────────────────────────
# Name      (no description)               0          0
# Price     (no description)               0          0
# Category  (no description)               0          0
# Stock     (no description)               0          0
# Notes     Additional notes or comments   0          0
```

**Error Cases**:
```bash
# Duplicate column
stash column add Name
# Error: column 'Name' already exists

# Reserved name
stash column add _id
# Error: '_id' is a reserved column name

# Invalid characters
stash column add "My Column"
# Error: column name cannot contain spaces
```

---

### UC-COL-002: List Columns

**Goal**: View all columns with descriptions and statistics.

```bash
stash column list
```

**Expected Output**:
```
Column    Description                     Populated  Empty
──────────────────────────────────────────────────────────────
Name      Item display name               100        0
Category  Product category                 85       15
Price     Price in USD                     72       28
Stock     Quantity in inventory            60       40
```

**JSON Output**:
```bash
stash column list --json
```
```json
[
  {"name": "Name", "desc": "Item display name", "populated": 100, "empty": 0},
  {"name": "Category", "desc": "Product category", "populated": 85, "empty": 15},
  {"name": "Price", "desc": "Price in USD", "populated": 72, "empty": 28}
]
```

**Multi-Stash**:
```bash
stash column list --stash contacts
```

---

### UC-COL-003: Describe a Column

**Goal**: Add or update a column description.

```bash
# Set description
stash column describe Price "Price in USD, excluding tax"
# Updated description for column 'Price'

# Verify
stash column list
# Price     Price in USD, excluding tax    72       28
```

**Use Case**: Help Claude understand what data goes in each column.

---

### UC-COL-004: Rename a Column

**Goal**: Rename a column while preserving data.

```bash
stash column rename Cost Price
# Renamed column 'Cost' to 'Price'
```

**Verification**:
```bash
# Old column gone, new column has data
stash column list
# Price (was Cost)...

stash list --json | head -1 | jq .
# { "Price": 999 }  # Data preserved
```

**Error Cases**:
```bash
# Target exists
stash column rename Price Name
# Error: column 'Name' already exists
```

---

### UC-COL-005: Drop a Column

**Goal**: Remove a column from schema.

```bash
# With confirmation
stash column drop OldField
# This will remove column 'OldField' from the schema.
# Data will be preserved in JSONL history.
# Are you sure? [y/N] y
# Dropped column 'OldField'

# Skip confirmation
stash column drop OldField --yes
```

**Note**: Data is preserved in JSONL history but no longer queryable.

---

## 3. Record Operations

### UC-REC-001: Add a Record

**Goal**: Create a new record with auto-generated ID.

```bash
# Simple add (value goes to first column)
stash add "Laptop"
# inv-ex4j

# Add with additional fields
stash add "Laptop" --set Price 999 --set Category "electronics"
# inv-8t5n

# JSON output
stash add "Phone" --json
# {"_id":"inv-k2m9","_hash":"a1b2c3d4e5f6","_created_by":"alice","Name":"Phone"}
```

**Verification**:
```bash
stash show inv-ex4j
# Record: inv-ex4j
# Hash: a1b2c3d4e5f6
# Created: 2025-01-08 10:00:00 by alice (main)
#
# Fields:
#   Name: Laptop
```

---

### UC-REC-002: Add Child Records (Hierarchy)

**Goal**: Create hierarchical data with parent-child relationships.

```bash
# Create parent
stash add "Electronics"
# inv-a1b2

# Create children
stash add "Computers" --parent inv-a1b2
# inv-a1b2.1

stash add "Phones" --parent inv-a1b2
# inv-a1b2.2

# Create grandchild
stash add "MacBook Pro" --parent inv-a1b2.1
# inv-a1b2.1.1
```

**View Hierarchy**:
```bash
stash list --tree
# inv-a1b2   Electronics
# ├─ inv-a1b2.1   Computers
# │  └─ inv-a1b2.1.1   MacBook Pro
# └─ inv-a1b2.2   Phones
```

**Error Cases**:
```bash
# Invalid parent
stash add "Orphan" --parent inv-fake
# Error: parent record 'inv-fake' not found
```

---

### UC-REC-003: Update Record Fields

**Goal**: Modify existing record data.

```bash
# Update single field
stash set inv-ex4j Price 1299
# Updated inv-ex4j

# Update multiple fields
stash set inv-ex4j --col Price 1299 --col Stock 50
# Updated inv-ex4j
```

**Verification**:
```bash
stash show inv-ex4j --json | jq '{Price, Stock}'
# {"Price": 1299, "Stock": 50}
```

**Error Cases**:
```bash
# Non-existent record
stash set inv-fake Price 100
# Error: record 'inv-fake' not found

# Non-existent column
stash set inv-ex4j FakeColumn "value"
# Error: column 'FakeColumn' not found

# Deleted record
stash set inv-deleted Price 100
# Error: record 'inv-deleted' is deleted. Use 'stash restore' first.
```

---

### UC-REC-004: Show Record Details

**Goal**: View a single record with all fields and metadata.

```bash
stash show inv-ex4j
```

**Expected Output**:
```
Record: inv-ex4j
Hash: a1b2c3d4e5f6
Parent: (none)
Created: 2025-01-08 10:30:00 by alice (main)
Updated: 2025-01-08 11:10:00 by bob (feature-prices)

Fields:
  Name: Laptop
  Category: electronics
  Price: 1299

Children:
  inv-ex4j.1  Charger
  inv-ex4j.2  Laptop Bag
```

**With File Contents**:
```bash
stash show inv-ex4j --with-files
# ... includes full markdown content from attached files
```

**With History**:
```bash
stash show inv-ex4j --history
# Change History:
#   2025-01-08 10:30:00  create  alice  main           Name="Laptop"
#   2025-01-08 10:45:00  update  alice  main           Category="electronics"
#   2025-01-08 11:10:00  update  bob    feature-prices Price=1299
```

**JSON Output**:
```bash
stash show inv-ex4j --json
```

---

### UC-REC-005: Attach Markdown Files

**Goal**: Create and attach markdown content to a record.

```bash
# Inline content
stash file inv-ex4j Description --content "# Laptop

High-performance laptop for developers.

## Specifications
- 16GB RAM
- 512GB SSD
"
# Created .stash/inventory/files/inv-ex4j.md
# Updated inv-ex4j.Description = "inv-ex4j.md"

# From existing file
stash file inv-ex4j Description --from ./docs/laptop-specs.md
```

**Verification**:
```bash
cat .stash/inventory/files/inv-ex4j.md
# (file contents)

stash show inv-ex4j --with-files
# (includes file contents inline)
```

---

### UC-REC-006: Delete Records (Soft)

**Goal**: Soft-delete records (can be restored).

```bash
# Simple delete
stash delete inv-ex4j
# Deleted inv-ex4j

# With confirmation skip
stash delete inv-ex4j --yes

# Cascade delete (with children)
stash delete inv-a1b2 --cascade
# Deleted inv-a1b2 and 3 children
```

**Verification**:
```bash
# Not in normal list
stash list
# (inv-ex4j not shown)

# Shows in deleted list
stash list --deleted
# inv-ex4j  Laptop  (deleted 2025-01-08 by alice)
```

**Error Cases**:
```bash
# Has children without --cascade
stash delete inv-a1b2
# Error: record has 2 children. Use --cascade to delete all.
```

---

### UC-REC-007: Restore Deleted Records

**Goal**: Restore soft-deleted records.

```bash
stash restore inv-ex4j
# Restored inv-ex4j

# With cascade
stash restore inv-a1b2 --cascade
# Restored inv-a1b2 and 3 children
```

**Verification**:
```bash
stash list
# (inv-ex4j now visible)
```

---

### UC-REC-008: Purge Deleted Records

**Goal**: Permanently remove soft-deleted records.

```bash
# Purge by age
stash purge --before 30d
# Will permanently remove 5 records deleted > 30 days ago.
# Continue? [y/N] y
# Purged 5 records

# Purge specific record
stash purge --id inv-ex4j --yes

# Purge all deleted
stash purge --all --yes

# Dry run (preview)
stash purge --before 7d --dry-run
# Would purge:
#   inv-old1  deleted 2024-12-01 by alice
#   inv-old2  deleted 2024-12-05 by bob
```

**Warning**: Purged records cannot be recovered.

---

## 4. Querying

### UC-QRY-001: List Records

**Goal**: List records with filtering and output options.

```bash
# All records
stash list

# Filter with WHERE
stash list --where "Category = 'electronics'"
stash list --where "Price > 500"
stash list --where "Stock IS NULL"
stash list --where "Name LIKE '%Laptop%'"

# Select columns
stash list --columns "id,Name,Price"

# Tree view
stash list --tree

# JSON output (for agents)
stash list --json

# Show only deleted
stash list --deleted
```

**Example Output (table)**:
```
ID        Name       Category     Price  Updated
──────────────────────────────────────────────────
inv-ex4j  Laptop     electronics  1299   bob (2h ago)
inv-8t5n  Phone      electronics   999   alice (1d ago)
inv-k2m9  Notebook   office         12   alice (3d ago)
```

**Example Output (JSON)**:
```json
[
  {"_id": "inv-ex4j", "Name": "Laptop", "Category": "electronics", "Price": 1299},
  {"_id": "inv-8t5n", "Name": "Phone", "Category": "electronics", "Price": 999}
]
```

---

### UC-QRY-002: List Children

**Goal**: List direct children of a record.

```bash
stash children inv-a1b2
# ID           Name        Category
# ─────────────────────────────────
# inv-a1b2.1   Computers   subcategory
# inv-a1b2.2   Phones      subcategory

stash children inv-a1b2 --json
```

---

### UC-QRY-003: Raw SQL Query

**Goal**: Execute arbitrary SELECT queries.

```bash
# Simple query
stash query "SELECT Name, Price FROM inventory WHERE Price > 500"

# Aggregation
stash query "SELECT Category, COUNT(*) as count FROM inventory GROUP BY Category"

# Join with ordering
stash query "SELECT * FROM inventory ORDER BY updated_at DESC LIMIT 10"

# JSON output
stash query "SELECT * FROM inventory" --json
```

**Error Cases**:
```bash
# Non-SELECT blocked
stash query "DELETE FROM inventory"
# Error: only SELECT queries are allowed
```

---

### UC-QRY-004: View Change History

**Goal**: Audit changes to records.

```bash
# All recent changes
stash history

# Specific record
stash history inv-ex4j

# Filter by actor
stash history --by alice

# Filter by time
stash history --since 24h
stash history --since 7d

# Combined
stash history --since 1w --by alice

# Limit results
stash history --limit 50

# JSON output
stash history --json
```

**Example Output**:
```
Recent changes:

2025-01-08 11:10:00  update  inv-ex4j   bob    feature-prices  Price=1299
2025-01-08 10:45:00  update  inv-ex4j   alice  main            Category="electronics"
2025-01-08 10:30:00  create  inv-ex4j   alice  main            Name="Laptop"
2025-01-08 10:25:00  create  inv-8t5n   alice  main            Name="Phone"
```

---

## 5. Import/Export

### UC-IMP-001: Import from CSV

**Goal**: Bulk import records from CSV file.

```bash
# Interactive import
stash import products.csv
```

**Interactive Output**:
```
Importing from products.csv

Detected columns:
  Name      (100 values, 0 empty) - exists
  Category  (98 values, 2 empty)  - NEW
  Price     (95 values, 5 empty)  - NEW

This will:
  - Add columns: Category, Price
  - Create 100 new records with prefix: inv-
  - Actor: alice
  - Branch: main

Proceed? [y/N]
```

**Non-Interactive**:
```bash
stash import products.csv --confirm

# Specify primary column
stash import products.csv --column ProductName --confirm

# Dry run
stash import products.csv --dry-run
```

**CSV Format**:
```csv
Name,Category,Price
Laptop,electronics,999
Phone,electronics,699
Notebook,office,12
```

---

### UC-IMP-002: Export to File

**Goal**: Export records to CSV or JSON.

```bash
# Export all to CSV
stash export products.csv

# Export to JSON
stash export products.json --format json

# Filtered export
stash export electronics.csv --where "Category = 'electronics'"

# Include deleted
stash export all-data.csv --include-deleted
```

---

## 6. Maintenance

### UC-SYN-001: Sync JSONL and SQLite

**Goal**: Ensure JSONL and SQLite cache are synchronized.

```bash
# Check status
stash sync --status
# Sync status:
#   inventory: in-sync (100 records)
#   contacts: 2 pending changes

# Normal sync
stash sync
# Synced all stashes

# Full rebuild from JSONL
stash sync --rebuild

# Flush DB changes to JSONL
stash sync --flush

# Pull from main branch (for worktrees)
stash sync --from-main
```

---

### UC-SYN-002: Health Check with Doctor

**Goal**: Diagnose and fix issues.

```bash
# Basic check
stash doctor
```

**Example Output**:
```
Stash Doctor

✓ Daemon running (PID 12345)
✓ Cache database valid
✓ inventory: 100 records, in-sync
✓ contacts: 25 records, in-sync
⚠ inventory: 3 file references point to missing files
⚠ contacts: Column 'Email' has no description
⚠ inventory: 2 records have hash mismatch

Issues found: 3 warnings, 0 errors

Run 'stash doctor --fix' to repair.
```

**Auto-Fix**:
```bash
stash doctor --fix --yes

# Deep check (slower, verifies hashes)
stash doctor --deep

# JSON output
stash doctor --json
```

---

### UC-SYN-003: Emergency Repair

**Goal**: Recover from corruption.

```bash
# Dry run
stash repair --dry-run

# Rebuild from JSONL (recommended)
stash repair --source jsonl

# Rebuild JSONL from DB
stash repair --source db

# Clean orphaned files
stash repair --clean-orphans

# Recalculate all hashes
stash repair --rehash
```

---

## 7. Daemon Management

### UC-DMN-001: Start Daemon

**Goal**: Start the background sync daemon.

```bash
stash daemon start
# Daemon started (PID 12345)
```

---

### UC-DMN-002: Stop Daemon

**Goal**: Stop the daemon gracefully.

```bash
stash daemon stop
# Daemon stopped
```

---

### UC-DMN-003: Restart Daemon

**Goal**: Restart the daemon.

```bash
stash daemon restart
# Daemon restarted (PID 12346)
```

---

### UC-DMN-004: Check Daemon Status

**Goal**: View daemon health.

```bash
stash daemon status
```

**Output**:
```
Daemon Status:
  PID: 12345
  Running: yes
  Uptime: 2h 15m
  Last sync: 3s ago
  Watched: 2 stashes
  Memory: 12 MB
```

---

### UC-DMN-005: View Daemon Logs

**Goal**: Debug daemon issues.

```bash
stash daemon logs
# (tails .stash/daemon.log)
```

---

## 8. End-to-End Workflows

### Workflow 1: Research Pipeline

**Scenario**: Import companies, research each, track findings.

```bash
# Setup
stash init research --prefix re-
stash import companies.csv --confirm
stash column add Verified Website CEO Overview

# Research loop
for id in $(stash list --json | jq -r '.[].id'); do
    name=$(stash show "$id" --json | jq -r '.Name')

    # Agent researches...

    stash set "$id" Verified true
    stash set "$id" Website "https://example.com"
    stash set "$id" CEO "Jane Doe"
    stash file "$id" Overview --content "# $name\n\n..."
done

# Query results
stash list --where "Verified = false"
stash export verified.csv --where "Verified = true"
```

---

### Workflow 2: Hierarchical Catalog

**Scenario**: Build a product catalog with categories.

```bash
# Setup
stash init catalog --prefix cat-
stash column add Name Type Price

# Create hierarchy
root=$(stash add "Electronics" --set Type category)
comp=$(stash add "Computers" --parent "$root" --set Type subcategory)
stash add "MacBook Pro" --parent "$comp" --set Type product --set Price 1999
stash add "ThinkPad" --parent "$comp" --set Type product --set Price 1499

phones=$(stash add "Phones" --parent "$root" --set Type subcategory)
stash add "iPhone 15" --parent "$phones" --set Type product --set Price 999

# View
stash list --tree
stash query "SELECT Name, Price FROM catalog WHERE Type = 'product' ORDER BY Price DESC"
```

---

### Workflow 3: Multi-Agent Collaboration

**Scenario**: Multiple agents working in worktrees.

```bash
# Main worktree: Agent 1
cd ~/project
stash list --where "Status = 'pending'" --json > /tmp/work.json
# Process items...

# Feature worktree: Agent 2
cd ~/worktrees/feature-xyz
stash sync --from-main  # Get latest
stash list --where "Assigned = 'agent2'" --json
# Process items...

# Before commit
stash sync --flush
git add .stash/*/records.jsonl

# After merge
cd ~/project
git pull
stash sync --rebuild
```

---

### Workflow 4: Recovery from Issues

**Scenario**: Something went wrong, need to diagnose and fix.

```bash
# Check daemon
stash daemon status
# Not running? Start it:
stash daemon start

# Run diagnostics
stash doctor --deep

# Found issues? Fix them:
stash doctor --fix --yes

# Still broken? Full rebuild:
stash repair --source jsonl

# Verify
stash doctor
stash list --json | wc -l
```

---

## Error Reference

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Stash not found |
| 4 | Record not found |
| 5 | Sync error |
| 6 | Hash verification failed |

---

## Coverage Checklist

### Stash Management
- [x] UC-ST-001: init
- [x] UC-ST-002: drop
- [x] UC-ST-003: info
- [x] UC-ST-004: onboard
- [x] UC-ST-005: prime

### Column Management
- [x] UC-COL-001: column add
- [x] UC-COL-002: column list
- [x] UC-COL-003: column describe
- [x] UC-COL-004: column rename
- [x] UC-COL-005: column drop

### Record Operations
- [x] UC-REC-001: add
- [x] UC-REC-002: add --parent (hierarchy)
- [x] UC-REC-003: set
- [x] UC-REC-004: show
- [x] UC-REC-005: file
- [x] UC-REC-006: delete
- [x] UC-REC-007: restore
- [x] UC-REC-008: purge

### Querying
- [x] UC-QRY-001: list
- [x] UC-QRY-002: children
- [x] UC-QRY-003: query
- [x] UC-QRY-004: history

### Import/Export
- [x] UC-IMP-001: import
- [x] UC-IMP-002: export

### Maintenance
- [x] UC-SYN-001: sync
- [x] UC-SYN-002: doctor
- [x] UC-SYN-003: repair

### Daemon
- [x] UC-DMN-001: daemon start
- [x] UC-DMN-002: daemon stop
- [x] UC-DMN-003: daemon restart
- [x] UC-DMN-004: daemon status
- [x] UC-DMN-005: daemon logs

**Total: 28 use cases covering all commands**
