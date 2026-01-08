# Stash: Technical Specification

**Version:** 2.1.0
**Language:** Go
**Concept:** A record-centric structured data store for AI agents

---

## 1. Overview

### What is Stash?

Stash is a lightweight, single-binary tool that provides AI agents with a structured way to collect, organize, and query any kind of data. Unlike scattered markdown files or unwieldy CSVs, Stash offers:

- **Fluid schema**: Add columns anytime without migrations
- **Hierarchical records**: Parent-child relationships with dot notation IDs
- **Dual storage**: JSONL source of truth + SQLite cache for queries
- **Full audit trail**: Track who created/modified records and when
- **Change detection**: Content hashing for integrity verification
- **Agent-native**: JSON output, context injection, conversational commands

### Design Philosophy

1. **Record-centric**: Records are primary; files are optional attachments
2. **Agent-first**: Commands map to natural language instructions
3. **Beads-compatible**: Same patterns (JSONL + SQLite + daemon), complementary purpose
4. **Human-readable**: Everything inspectable as plain files
5. **Git-aware**: Detect branch context when available, work without it

### Beads vs Stash

| Beads | Stash |
|-------|-------|
| Tracks **tasks/issues** (process) | Tracks **any structured data** (content) |
| Fixed schema | Fluid schema |
| Dependency graph | Hierarchical records |
| `bd-a1b2` IDs | `<prefix>-a1b2` IDs |

### Example Use Cases

Stash can track any structured data:
- **Company research**: Companies, contacts, competitors
- **Inventory**: Products, categories, suppliers
- **Content**: Articles, bookmarks, notes, sources
- **Projects**: Tasks, milestones, deliverables
- **Entities**: People, places, organizations

---

## 2. Required System Fields

Every record has these mandatory system fields (prefixed with `_`):

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | Unique record identifier (e.g., `inv-g7ewn`) |
| `_hash` | string | SHA-256 hash of record content (for change detection) |
| `_created_at` | timestamp | UTC timestamp when record was created |
| `_created_by` | string | Actor who created the record |
| `_updated_at` | timestamp | UTC timestamp of last modification |
| `_updated_by` | string | Actor who last modified the record |
| `_branch` | string | Git branch where record was created/modified (if available) |
| `_parent` | string | Parent record ID (optional, for hierarchy) |
| `_deleted_at` | timestamp | UTC timestamp when soft-deleted (null if active) |
| `_deleted_by` | string | Actor who deleted the record (null if active) |

### Hash Calculation

The `_hash` field is a SHA-256 hash of the record's user data (excluding system fields):

```go
func calculateHash(fields map[string]interface{}) string {
    // 1. Extract only user fields (exclude _ prefixed)
    userFields := make(map[string]interface{})
    for k, v := range fields {
        if !strings.HasPrefix(k, "_") {
            userFields[k] = v
        }
    }

    // 2. Sort keys for deterministic ordering
    keys := make([]string, 0, len(userFields))
    for k := range userFields {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    // 3. Build canonical JSON
    var buf bytes.Buffer
    for _, k := range keys {
        v, _ := json.Marshal(userFields[k])
        buf.WriteString(k)
        buf.WriteString(":")
        buf.Write(v)
        buf.WriteString("\n")
    }

    // 4. SHA-256 hash, return first 12 chars
    hash := sha256.Sum256(buf.Bytes())
    return hex.EncodeToString(hash[:])[:12]
}
```

**Use cases for hash:**
- Detect if record content changed
- Identify duplicate records
- Verify data integrity after sync
- Track changes across branches

### Actor Resolution

The actor (for `_created_by`, `_updated_by`, `_deleted_by`) is determined in order:

1. `--actor` flag on command
2. `$STASH_ACTOR` environment variable
3. `$USER` environment variable
4. `"unknown"`

### Branch Detection

Git branch is captured automatically when available:

```go
func detectBranch() string {
    cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
    out, err := cmd.Output()
    if err != nil {
        return ""  // Not in a git repo, or git not available
    }
    return strings.TrimSpace(string(out))
}
```

If not in a git repository, `_branch` is empty string (not an error).

---

## 3. Architecture

### Storage Layout

```
.stash/
├── cache.db                     # SQLite cache (daemon-managed)
├── daemon.pid                   # Daemon process ID
├── daemon.log                   # Daemon log file
├── inventory/                   # Stash: "inventory"
│   ├── config.json              # Schema + metadata
│   ├── records.jsonl            # Append-only source of truth
│   └── files/                   # Attached markdown files
│       ├── inv-ex4j.md
│       └── inv-ex4j.1.md
└── contacts/                    # Stash: "contacts"
    ├── config.json
    ├── records.jsonl
    └── files/
```

### config.json

```json
{
  "name": "inventory",
  "prefix": "inv-",
  "created": "2025-01-08T10:30:00Z",
  "created_by": "alice",
  "columns": [
    {
      "name": "Name",
      "desc": "Item display name",
      "added": "2025-01-08T10:30:00Z",
      "added_by": "alice"
    },
    {
      "name": "Category",
      "desc": "Product category (electronics, clothing, etc.)",
      "added": "2025-01-08T11:00:00Z",
      "added_by": "alice"
    },
    {
      "name": "Price",
      "desc": "Price in USD",
      "added": "2025-01-08T11:00:00Z",
      "added_by": "bob"
    }
  ]
}
```

### records.jsonl

Each line is a complete operation record (append-only log):

```jsonl
{"_id":"inv-ex4j","_hash":"a1b2c3d4e5f6","_op":"create","_created_at":"2025-01-08T10:30:00Z","_created_by":"alice","_updated_at":"2025-01-08T10:30:00Z","_updated_by":"alice","_branch":"main","Name":"Laptop"}
{"_id":"inv-ex4j","_hash":"b2c3d4e5f6g7","_op":"update","_updated_at":"2025-01-08T11:00:00Z","_updated_by":"bob","_branch":"feature-prices","Price":999}
{"_id":"inv-ex4j.1","_hash":"c3d4e5f6g7h8","_op":"create","_created_at":"2025-01-08T11:05:00Z","_created_by":"alice","_updated_at":"2025-01-08T11:05:00Z","_updated_by":"alice","_branch":"main","_parent":"inv-ex4j","Name":"Laptop Charger"}
{"_id":"inv-8t5n","_hash":"d4e5f6g7h8i9","_op":"delete","_deleted_at":"2025-01-08T12:00:00Z","_deleted_by":"alice","_branch":"main"}
```

### SQLite Schema

```sql
-- One table per stash (created dynamically)
CREATE TABLE inventory (
    id TEXT PRIMARY KEY,
    hash TEXT NOT NULL,
    parent_id TEXT,
    created_at TEXT NOT NULL,
    created_by TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    branch TEXT,
    deleted_at TEXT,
    deleted_by TEXT,
    Name TEXT,
    Category TEXT,
    Price TEXT
    -- User columns added dynamically via ALTER TABLE
);

CREATE INDEX idx_inventory_parent ON inventory(parent_id);
CREATE INDEX idx_inventory_deleted ON inventory(deleted_at);
CREATE INDEX idx_inventory_hash ON inventory(hash);
CREATE INDEX idx_inventory_branch ON inventory(branch);
CREATE INDEX idx_inventory_updated ON inventory(updated_at);

-- Metadata table (shared)
CREATE TABLE _stash_meta (
    stash_name TEXT PRIMARY KEY,
    prefix TEXT,
    config_json TEXT,
    last_sync TEXT
);
```

---

## 4. ID Generation

### Format

IDs follow the pattern: `<prefix>-<random>` or `<prefix>-<random>.<seq>` for children.

```
inv-ex4j          # Root record
inv-ex4j.1        # First child
inv-ex4j.2        # Second child
inv-ex4j.1.1      # Grandchild (child of inv-ex4j.1)
```

### Algorithm

1. **Root ID**: `<prefix>-<4-char-base36>`
   - Base36: `[0-9a-z]` (36 chars)
   - 4 chars = 1.6M combinations per prefix
   - Check for collision, regenerate if exists

2. **Child ID**: `<parent-id>.<next-seq>`
   - Sequential within parent
   - Query: `SELECT MAX(seq) FROM records WHERE parent_id = ?`

### Examples

```bash
stash add "Laptop"                    → inv-ex4j
stash add "Charger" --parent inv-ex4j → inv-ex4j.1
stash add "Phone"                     → inv-8t5n
stash add "Case" --parent inv-8t5n    → inv-8t5n.1
```

---

## 5. CLI Reference

### Global Flags

```
--json              Output in JSON format (for agent parsing)
--stash <name>      Target specific stash (default: auto-detect or $STASH_DEFAULT)
--actor <name>      Override actor for audit trail (default: $STASH_ACTOR or $USER)
--quiet             Suppress non-essential output
--verbose           Enable debug output
--no-daemon         Bypass daemon, direct file access
```

### Setup & Integration

#### `stash init`

Create a new stash and start daemon.

```bash
stash init <name> --prefix <prefix>

# Examples
stash init inventory --prefix inv-
stash init contacts --prefix ct-
stash init bookmarks --prefix bk-

# Flags
--prefix <p>    Required. 2-4 char prefix for IDs
--no-daemon     Don't auto-start daemon
```

Output:
```
Created stash 'inventory' with prefix 'inv-'
Location: .stash/inventory/
Actor: alice
Branch: main
Daemon started (PID 12345)

Run 'stash onboard' to get CLAUDE.md snippet.
```

#### `stash drop`

Delete a stash and all its data permanently.

```bash
stash drop <name> [--yes]

# Example
stash drop old-project --yes
```

**Warning**: This permanently deletes the stash directory including all records.jsonl history. Use `stash delete` for soft-deleting individual records.

#### `stash info`

Show status of all stashes.

```bash
stash info [--json]
```

Output:
```
Stashes:
  inventory (inv-)  100 records (3 deleted), 47 files, synced
  contacts (ct-)     25 records (0 deleted),  0 files, synced

Actor: alice
Branch: feature-xyz
Daemon: running (PID 12345), last sync 2s ago
Cache: .stash/cache.db (2.1 MB)
```

#### `stash onboard`

Output CLAUDE.md snippet for agent integration.

```bash
stash onboard
```

Output:
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

#### `stash prime`

Output current context for agent injection.

```bash
stash prime [--stash <name>]
```

Output:
```markdown
# Stash Context

Actor: alice
Branch: feature-xyz

## Active Stashes

### inventory (prefix: inv-)
Columns:
  - Name: Item display name
  - Category: Product category (electronics, clothing, etc.)
  - Price: Price in USD

Records: 100 total (3 soft-deleted)
  - 47 in Category = 'electronics'
  - 32 with Price set

Recent changes (last 24h):
  - 5 created by alice
  - 2 updated by bob

### contacts (prefix: ct-)
Columns:
  - Name: Full name
  - Email: Email address
  - Company: Company name

Records: 25 total (0 soft-deleted)

## Quick Commands
- Iterate: `stash list --json | jq -c '.[]'`
- Filter: `stash list --where "Category = 'electronics'" --json`
- Update: `stash set <id> <column> <value>`
- Delete: `stash delete <id>`
- Undelete: `stash restore <id>`
```

---

### Schema Management

#### `stash column add`

Add one or more columns to a stash.

```bash
stash column add <name>... [--desc "description"] [--stash <name>]

# Examples
stash column add Name
stash column add Category Price Stock
stash column add Notes --desc "Additional notes or comments"
```

#### `stash column list`

List columns with descriptions and statistics.

```bash
stash column list [--stash <name>] [--json]
```

Output (table):
```
Column    Description                                   Populated  Empty
──────────────────────────────────────────────────────────────────────────
Name      Item display name                             100        0
Category  Product category (electronics, clothing...)    85       15
Price     Price in USD                                   72       28
Stock     Quantity in inventory                          60       40
```

Output (JSON):
```json
[
  {"name": "Name", "desc": "Item display name", "populated": 100, "empty": 0},
  {"name": "Category", "desc": "Product category (electronics, clothing...)", "populated": 85, "empty": 15}
]
```

#### `stash column describe`

Set or update a column description.

```bash
stash column describe <name> "description"

# Example
stash column describe Price "Price in USD, excluding tax"
```

#### `stash column rename`

Rename a column.

```bash
stash column rename <old-name> <new-name>

# Example
stash column rename Cost Price
```

#### `stash column drop`

Remove a column from schema (data preserved in JSONL history).

```bash
stash column drop <name> [--yes]
```

---

### Record Operations

#### `stash add`

Create a new record.

```bash
stash add <primary-value> [--parent <id>] [--set <col> <val>]... [--stash <name>]

# Examples
stash add "Laptop"
stash add "Charger" --parent inv-ex4j
stash add "Phone" --set Category "electronics" --set Price 999

# Output
inv-ex4j

# Output (--json)
{"_id": "inv-ex4j", "_hash": "a1b2c3d4e5f6", "_created_by": "alice", "_branch": "main", "Name": "Laptop"}
```

The primary value goes into the first column defined in the schema.

#### `stash set`

Update a field on a record.

```bash
stash set <id> <column> <value>

# Examples
stash set inv-ex4j Price 1299
stash set inv-ex4j Category "electronics"
stash set inv-ex4j.1 Stock 50

# Multiple columns
stash set inv-ex4j --col Price 1299 --col Stock 25
```

#### `stash file`

Create or attach a markdown file to a record.

```bash
stash file <id> <column> [--content "..."] [--from <path>]

# Create with inline content
stash file inv-ex4j Description --content "# Laptop

High-performance laptop for developers.

## Specifications
- 16GB RAM
- 512GB SSD
- 14\" display
"

# Copy from existing file
stash file inv-ex4j Description --from ./docs/laptop-specs.md

# Output
Created .stash/inventory/files/inv-ex4j.md
Updated inv-ex4j.Description = "inv-ex4j.md"
```

#### `stash show`

Display a record with all fields.

```bash
stash show <id> [--json] [--with-files] [--history]

# Examples
stash show inv-ex4j
stash show inv-ex4j --json
stash show inv-ex4j --with-files  # Include file contents
stash show inv-ex4j --history     # Show change history
```

Output (table):
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
  Description: inv-ex4j.md

Children:
  inv-ex4j.1  Charger
  inv-ex4j.2  Laptop Bag
```

Output (JSON):
```json
{
  "_id": "inv-ex4j",
  "_hash": "a1b2c3d4e5f6",
  "_parent": null,
  "_created_at": "2025-01-08T10:30:00Z",
  "_created_by": "alice",
  "_updated_at": "2025-01-08T11:10:00Z",
  "_updated_by": "bob",
  "_branch": "feature-prices",
  "Name": "Laptop",
  "Category": "electronics",
  "Price": 1299,
  "Description": "inv-ex4j.md",
  "_children": ["inv-ex4j.1", "inv-ex4j.2"]
}
```

Output with `--history`:
```
Record: inv-ex4j

Change History:
  2025-01-08 10:30:00  create  alice  main           Name="Laptop"
  2025-01-08 10:45:00  update  alice  main           Category="electronics"
  2025-01-08 11:10:00  update  bob    feature-prices Price=1299
```

#### `stash delete`

Soft-delete a record (can be restored).

```bash
stash delete <id> [--cascade] [--yes]

# Flags
--cascade    Also delete all children
--yes        Skip confirmation

# Example
stash delete inv-ex4j --cascade
```

Soft-deleted records:
- Have `_deleted_at` and `_deleted_by` set
- Are excluded from `stash list` by default
- Can be restored with `stash restore`
- Can be permanently removed with `stash purge`

#### `stash restore`

Restore a soft-deleted record.

```bash
stash restore <id> [--cascade]

# Flags
--cascade    Also restore all deleted children

# Example
stash restore inv-ex4j --cascade
```

#### `stash purge`

Permanently remove soft-deleted records.

```bash
stash purge [--before <duration>] [--id <id>] [--all] [--dry-run] [--yes]

# Flags
--before <duration>   Purge records deleted before this duration (e.g., 30d, 1w, 24h)
--id <id>             Purge specific record by ID
--all                 Purge all soft-deleted records
--dry-run             Show what would be purged without doing it
--yes                 Skip confirmation

# Examples
stash purge --before 30d                    # Purge records deleted > 30 days ago
stash purge --before 1w --dry-run           # Preview what would be purged
stash purge --id inv-ex4j --yes             # Purge specific record
stash purge --all --yes                     # Purge all deleted records
```

Output:
```
Purging soft-deleted records older than 30 days...

Will permanently remove:
  inv-8t5n   deleted 2024-12-01 by alice
  inv-k2m9   deleted 2024-12-05 by bob
  inv-p3q4   deleted 2024-12-10 by alice

3 records will be permanently removed. Continue? [y/N]
```

**Warning**: Purged records cannot be recovered. The JSONL entries are removed and files deleted.

---

### Querying

#### `stash list`

List records with optional filtering.

```bash
stash list [--where "..."] [--columns "..."] [--tree] [--json] [--deleted] [--stash <name>]

# Examples
stash list
stash list --where "Category = 'electronics'"
stash list --where "Price > 500"
stash list --where "updated_by = 'alice'"
stash list --columns "id,Name,Price"
stash list --tree
stash list --json
stash list --deleted           # Show only deleted records
stash list --deleted --all     # Show both active and deleted
```

Output (table):
```
ID        Name       Category     Price  Updated
──────────────────────────────────────────────────
inv-ex4j  Laptop     electronics  1299   bob (2h ago)
inv-8t5n  Phone      electronics   999   alice (1d ago)
inv-k2m9  Notebook   office         12   alice (3d ago)
```

Output (tree):
```
inv-ex4j   Laptop
├─ inv-ex4j.1   Charger
│  └─ inv-ex4j.1.1   USB-C Cable
└─ inv-ex4j.2   Laptop Bag
inv-8t5n   Phone
└─ inv-8t5n.1   Phone Case
inv-k2m9   Notebook
```

Output (JSON):
```json
[
  {"_id": "inv-ex4j", "_hash": "a1b2c3d4e5f6", "Name": "Laptop", "Category": "electronics", "Price": 1299},
  {"_id": "inv-8t5n", "_hash": "b2c3d4e5f6g7", "Name": "Phone", "Category": "electronics", "Price": 999}
]
```

#### `stash children`

List direct children of a record.

```bash
stash children <id> [--json]

# Example
stash children inv-ex4j
```

#### `stash query`

Execute raw SQL against the cache.

```bash
stash query "<sql>" [--json]

# Examples
stash query "SELECT Name, Price FROM inventory WHERE Category = 'electronics'"
stash query "SELECT COUNT(*) as total FROM inventory WHERE deleted_at IS NULL"
stash query "SELECT * FROM inventory WHERE Name LIKE '%Laptop%'"
stash query "SELECT created_by, COUNT(*) FROM inventory GROUP BY created_by"
```

#### `stash history`

Show change history for the stash or a specific record.

```bash
stash history [<id>] [--limit N] [--since <duration>] [--by <actor>] [--json]

# Examples
stash history                           # All recent changes
stash history inv-ex4j                  # Changes to specific record
stash history --limit 50                # Last 50 changes
stash history --since 24h               # Changes in last 24 hours
stash history --by alice                # Changes by specific actor
stash history --since 1w --by alice     # Combined filters
```

Output:
```
Recent changes:

2025-01-08 11:10:00  update  inv-ex4j   bob    feature-prices  Price=1299
2025-01-08 10:45:00  update  inv-ex4j   alice  main            Category="electronics"
2025-01-08 10:30:00  create  inv-ex4j   alice  main            Name="Laptop"
2025-01-08 10:25:00  create  inv-8t5n   alice  main            Name="Phone"
```

---

### Import/Export

#### `stash import`

Import records from CSV.

```bash
stash import <file.csv> [--stash <name>] [--column <primary>] [--confirm] [--dry-run]

# Examples
stash import products.csv
stash import products.csv --column Name
stash import products.csv --dry-run
stash import products.csv --confirm  # Skip interactive prompt
```

Workflow:
1. Parse CSV headers
2. Show column preview with sample data
3. Ask for confirmation (unless `--confirm`)
4. Create missing columns
5. Import records

Output:
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

#### `stash export`

Export records to CSV or JSON.

```bash
stash export <file> [--stash <name>] [--where "..."] [--format csv|json] [--include-deleted]

# Examples
stash export products.csv
stash export products.json --format json
stash export electronics.csv --where "Category = 'electronics'"
stash export all-data.csv --include-deleted
```

---

### Maintenance

#### `stash sync`

Synchronize JSONL and SQLite cache.

```bash
stash sync [--rebuild] [--flush] [--status] [--from-main]

# Flags
--rebuild      Full rebuild of SQLite from JSONL (source of truth)
--flush        Export any pending DB changes to JSONL
--status       Show sync state without making changes
--from-main    Pull JSONL changes from main branch (for worktrees)
```

Output:
```
Sync status:
  inventory: in-sync (100 records, hash verified)
  contacts: 2 pending changes

Syncing...
  Flushed 2 changes to contacts/records.jsonl
  All stashes synchronized.
```

#### `stash doctor`

Health check and diagnostics.

```bash
stash doctor [--fix] [--deep] [--json] [--yes]

# Flags
--fix     Automatically fix issues
--deep    Full validation including hash verification (slower)
--json    Machine-readable output
--yes     Skip confirmation for fixes
```

Checks performed:
- JSONL ↔ SQLite consistency
- Schema matches config.json
- File references valid
- Hierarchy integrity (parents exist)
- No orphaned files
- Daemon health
- Column descriptions present
- **Hash verification** (with `--deep`)

Output:
```
Stash Doctor

✓ Daemon running (PID 12345)
✓ Cache database valid
✓ inventory: 100 records, in-sync
✓ contacts: 25 records, in-sync
⚠ inventory: 3 file references point to missing files
⚠ contacts: Column 'Email' has no description
⚠ inventory: 2 records have hash mismatch (data may have been modified externally)

Issues found: 3 warnings, 0 errors

Run 'stash doctor --fix' to repair.
```

#### `stash repair`

Emergency repair for corrupted data.

```bash
stash repair [--dry-run] [--source jsonl|db] [--clean-orphans] [--rehash]

# Flags
--dry-run         Preview repairs without making changes
--source jsonl    Force rebuild from JSONL (recommended)
--source db       Force rebuild JSONL from DB
--clean-orphans   Remove orphaned files in files/
--rehash          Recalculate hashes for all records
```

---

### Daemon

#### `stash daemon`

Manage the background sync daemon.

```bash
stash daemon start    # Start daemon
stash daemon stop     # Stop daemon
stash daemon restart  # Restart daemon
stash daemon status   # Show status
stash daemon logs     # Tail log file
```

Status output:
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

## 6. Daemon Design

### Responsibilities

1. **Watch JSONL files** for external changes (git pull, manual edits)
2. **Update SQLite cache** when JSONL changes
3. **Handle concurrent access** from multiple processes
4. **Log operations** for debugging

### File Watching

Uses `fsnotify` to watch:
- `.stash/*/records.jsonl`
- `.stash/*/config.json`

On change:
1. Debounce (100ms) to batch rapid changes
2. Parse changed JSONL entries since last sync
3. Apply to SQLite cache
4. Update `last_sync` in metadata

### Process Management

```
.stash/daemon.pid    # Contains PID
.stash/daemon.log    # Rolling log (max 10MB)
.stash/daemon.sock   # Unix socket for IPC (optional)
```

Auto-start on `stash init`. Auto-stop on `stash drop` of last stash.

### Graceful Degradation

If daemon is not running:
- Commands still work (direct JSONL + SQLite access)
- Warning shown: "Daemon not running. Run 'stash daemon start' for auto-sync."
- `--no-daemon` flag skips daemon entirely

---

## 7. Go Implementation

### Package Structure

```
cmd/
  stash/
    main.go              # CLI entry point
internal/
  config/
    config.go            # Config parsing, validation
  storage/
    jsonl.go             # JSONL read/write
    sqlite.go            # SQLite operations
    sync.go              # JSONL ↔ SQLite sync
  model/
    record.go            # Record type, ID generation, hash calculation
    column.go            # Column type, schema
    stash.go             # Stash type
  context/
    context.go           # Actor resolution, branch detection
  daemon/
    daemon.go            # Daemon process management
    watcher.go           # File watching
  cli/
    root.go              # Root command, global flags
    init.go              # stash init
    add.go               # stash add
    set.go               # stash set
    list.go              # stash list
    delete.go            # stash delete, restore, purge
    history.go           # stash history
    ...
  output/
    json.go              # JSON formatting
    table.go             # Table formatting
    tree.go              # Tree formatting
```

### Key Dependencies

```go
require (
    github.com/spf13/cobra v1.8.0       // CLI framework
    modernc.org/sqlite v1.28.0          // Pure Go SQLite
    github.com/fsnotify/fsnotify v1.7.0 // File watching
    github.com/olekukonko/tablewriter   // Table output
)
```

### Core Types

```go
// model/record.go
type Record struct {
    ID         string                 `json:"_id"`
    Hash       string                 `json:"_hash"`
    ParentID   string                 `json:"_parent,omitempty"`
    CreatedAt  time.Time              `json:"_created_at"`
    CreatedBy  string                 `json:"_created_by"`
    UpdatedAt  time.Time              `json:"_updated_at"`
    UpdatedBy  string                 `json:"_updated_by"`
    Branch     string                 `json:"_branch,omitempty"`
    DeletedAt  *time.Time             `json:"_deleted_at,omitempty"`
    DeletedBy  string                 `json:"_deleted_by,omitempty"`
    Operation  string                 `json:"_op"`
    Fields     map[string]interface{} `json:"-"` // Flattened in JSON
}

func (r *Record) CalculateHash() string
func (r *Record) IsDeleted() bool

// model/stash.go
type Stash struct {
    Name      string    `json:"name"`
    Prefix    string    `json:"prefix"`
    Created   time.Time `json:"created"`
    CreatedBy string    `json:"created_by"`
    Columns   []Column  `json:"columns"`
}

// model/column.go
type Column struct {
    Name    string    `json:"name"`
    Desc    string    `json:"desc"`
    Added   time.Time `json:"added"`
    AddedBy string    `json:"added_by"`
}

// context/context.go
type Context struct {
    Actor  string
    Branch string
}

func NewContext() *Context
func (c *Context) ResolveActor(flagValue string) string
func (c *Context) DetectBranch() string
```

---

## 8. Error Handling

### Error Types

```go
var (
    ErrStashNotFound     = errors.New("stash not found")
    ErrStashExists       = errors.New("stash already exists")
    ErrRecordNotFound    = errors.New("record not found")
    ErrRecordDeleted     = errors.New("record is deleted")
    ErrColumnNotFound    = errors.New("column not found")
    ErrColumnExists      = errors.New("column already exists")
    ErrInvalidID         = errors.New("invalid record ID")
    ErrParentNotFound    = errors.New("parent record not found")
    ErrDaemonNotRunning  = errors.New("daemon not running")
    ErrSyncConflict      = errors.New("sync conflict detected")
    ErrHashMismatch      = errors.New("hash mismatch detected")
)
```

### Exit Codes

```
0   Success
1   General error
2   Invalid arguments
3   Stash not found
4   Record not found
5   Sync error
6   Hash verification failed
```

---

## 9. Configuration

### Environment Variables

```bash
STASH_DIR=.stash           # Stash directory location
STASH_DEFAULT=inventory    # Default stash for commands
STASH_ACTOR=alice          # Default actor for audit trail
STASH_NO_DAEMON=1          # Disable daemon auto-start
STASH_LOG_LEVEL=debug      # Log verbosity
```

### Global Config (optional)

`~/.config/stash/config.json`:
```json
{
  "default_stash": "inventory",
  "default_actor": "alice",
  "daemon_auto_start": true,
  "log_level": "info"
}
```

---

## 10. Future Considerations (v2+)

- **Column types**: Optional type hints (string, int, bool, file, date)
- **Validation**: Column constraints (required, unique, regex)
- **Relationships**: Cross-stash references with foreign keys
- **Hooks**: Pre/post operation hooks
- **Encryption**: Encrypted columns for sensitive data
- **Remote sync**: Git-based sync like beads
- **Web UI**: Simple browser interface for viewing data
- **Merge conflict resolution**: Automatic conflict handling on branch merge

---

## Appendix A: JSONL Format Examples

### Create Record
```json
{"_id":"inv-ex4j","_hash":"a1b2c3d4e5f6","_op":"create","_created_at":"2025-01-08T10:30:00Z","_created_by":"alice","_updated_at":"2025-01-08T10:30:00Z","_updated_by":"alice","_branch":"main","Name":"Laptop"}
```

### Update Field
```json
{"_id":"inv-ex4j","_hash":"b2c3d4e5f6g7","_op":"update","_updated_at":"2025-01-08T11:00:00Z","_updated_by":"bob","_branch":"feature-prices","Price":1299,"Category":"electronics"}
```

### Create Child Record
```json
{"_id":"inv-ex4j.1","_hash":"c3d4e5f6g7h8","_op":"create","_created_at":"2025-01-08T11:05:00Z","_created_by":"alice","_updated_at":"2025-01-08T11:05:00Z","_updated_by":"alice","_branch":"main","_parent":"inv-ex4j","Name":"Charger"}
```

### Delete Record (soft)
```json
{"_id":"inv-8t5n","_hash":"d4e5f6g7h8i9","_op":"delete","_updated_at":"2025-01-08T12:00:00Z","_updated_by":"alice","_branch":"main","_deleted_at":"2025-01-08T12:00:00Z","_deleted_by":"alice"}
```

### Restore Record
```json
{"_id":"inv-8t5n","_hash":"e5f6g7h8i9j0","_op":"restore","_updated_at":"2025-01-08T13:00:00Z","_updated_by":"alice","_branch":"main","_deleted_at":null,"_deleted_by":null}
```

---

## Appendix B: SQLite Query Examples

```sql
-- All active records in a stash
SELECT * FROM inventory WHERE deleted_at IS NULL;

-- All deleted records
SELECT * FROM inventory WHERE deleted_at IS NOT NULL;

-- Filter by field
SELECT id, Name, Price FROM inventory WHERE Category = 'electronics' AND deleted_at IS NULL;

-- Count by actor
SELECT created_by, COUNT(*) as count FROM inventory GROUP BY created_by;

-- Recent changes
SELECT id, Name, updated_by, updated_at FROM inventory
WHERE updated_at > datetime('now', '-24 hours')
ORDER BY updated_at DESC;

-- Changes by branch
SELECT id, Name, branch FROM inventory WHERE branch = 'feature-xyz';

-- Records with hash (for verification)
SELECT id, Name, hash FROM inventory WHERE deleted_at IS NULL;

-- Hierarchy: find all descendants
WITH RECURSIVE descendants AS (
    SELECT id, parent_id, Name, 0 as depth
    FROM inventory WHERE id = 'inv-ex4j'
    UNION ALL
    SELECT r.id, r.parent_id, r.Name, d.depth + 1
    FROM inventory r
    JOIN descendants d ON r.parent_id = d.id
    WHERE r.deleted_at IS NULL
)
SELECT * FROM descendants;

-- Purge candidates (deleted > 30 days ago)
SELECT id, Name, deleted_at, deleted_by FROM inventory
WHERE deleted_at IS NOT NULL
AND deleted_at < datetime('now', '-30 days');
```
