# Stash: Technical Specification

**Version:** 2.0.0
**Language:** Go
**Concept:** A record-centric structured data store for AI agents

---

## 1. Overview

### What is Stash?

Stash is a lightweight, single-binary tool that provides AI agents with a structured way to collect, organize, and query research data. Unlike scattered markdown files or unwieldy CSVs, Stash offers:

- **Fluid schema**: Add columns anytime without migrations
- **Hierarchical records**: Parent-child relationships with dot notation IDs
- **Dual storage**: JSONL source of truth + SQLite cache for queries
- **Claude-native**: JSON output, context injection, conversational commands

### Design Philosophy

1. **Record-centric**: Records are primary; files are optional attachments
2. **Agent-first**: Commands map to natural language instructions
3. **Beads-compatible**: Same patterns (JSONL + SQLite + daemon), complementary purpose
4. **Human-readable**: Everything inspectable as plain files

### Beads vs Stash

| Beads | Stash |
|-------|-------|
| Tracks **tasks/issues** (process) | Tracks **research data** (content) |
| Fixed schema | Fluid schema |
| Dependency graph | Hierarchical records |
| `bd-a1b2` IDs | `<prefix>-a1b2` IDs |

---

## 2. Architecture

### Storage Layout

```
.stash/
├── cache.db                     # SQLite cache (daemon-managed)
├── daemon.pid                   # Daemon process ID
├── daemon.log                   # Daemon log file
├── research/                    # Stash: "research"
│   ├── config.json              # Schema + metadata
│   ├── records.jsonl            # Append-only source of truth
│   └── files/                   # Attached markdown files
│       ├── re-ex4j.md
│       └── re-ex4j.1.md
└── contacts/                    # Stash: "contacts"
    ├── config.json
    ├── records.jsonl
    └── files/
```

### config.json

```json
{
  "name": "research",
  "prefix": "re-",
  "created": "2025-01-08T10:30:00Z",
  "columns": [
    {
      "name": "CompanyName",
      "desc": "Official registered company name",
      "added": "2025-01-08T10:30:00Z"
    },
    {
      "name": "Verified",
      "desc": "true if company existence confirmed via official sources",
      "added": "2025-01-08T11:00:00Z"
    },
    {
      "name": "Overview",
      "desc": "Markdown file with company summary, history, key facts",
      "added": "2025-01-08T11:00:00Z"
    }
  ]
}
```

### records.jsonl

Each line is a complete record snapshot (append-only log):

```jsonl
{"_id":"re-ex4j","_ts":"2025-01-08T10:30:00Z","_op":"create","CompanyName":"Microsoft"}
{"_id":"re-ex4j","_ts":"2025-01-08T11:00:00Z","_op":"update","Verified":true}
{"_id":"re-ex4j.1","_ts":"2025-01-08T11:05:00Z","_op":"create","_parent":"re-ex4j","CompanyName":"Azure"}
{"_id":"re-ex4j","_ts":"2025-01-08T11:10:00Z","_op":"update","Overview":"re-ex4j.md"}
```

Reserved fields (prefixed with `_`):
- `_id`: Record identifier
- `_ts`: Timestamp of operation
- `_op`: Operation type (create, update, delete)
- `_parent`: Parent record ID (for hierarchy)
- `_deleted`: Soft delete marker

### SQLite Schema

```sql
-- One table per stash (created dynamically)
CREATE TABLE research (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    created_at TEXT,
    updated_at TEXT,
    deleted_at TEXT,
    CompanyName TEXT,
    Verified TEXT,
    Overview TEXT
    -- Columns added dynamically via ALTER TABLE
);

CREATE INDEX idx_research_parent ON research(parent_id);
CREATE INDEX idx_research_deleted ON research(deleted_at);

-- Metadata table (shared)
CREATE TABLE _stash_meta (
    stash_name TEXT PRIMARY KEY,
    prefix TEXT,
    config_json TEXT,
    last_sync TEXT
);
```

---

## 3. ID Generation

### Format

IDs follow the pattern: `<prefix>-<random>` or `<prefix>-<random>.<seq>` for children.

```
re-ex4j           # Root record
re-ex4j.1         # First child
re-ex4j.2         # Second child
re-ex4j.1.1       # Grandchild (child of re-ex4j.1)
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

```
stash add "Microsoft"              → re-ex4j
stash add "Azure" --parent re-ex4j → re-ex4j.1
stash add "AWS"                    → re-8t5n
stash add "Lambda" --parent re-8t5n → re-8t5n.1
```

---

## 4. CLI Reference

### Global Flags

```
--json              Output in JSON format (for agent parsing)
--stash <name>      Target specific stash (default: auto-detect)
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
stash init research --prefix re-
stash init contacts --prefix ct-

# Flags
--prefix <p>    Required. 2-4 char prefix for IDs
--no-daemon     Don't auto-start daemon
```

Output:
```
Created stash 'research' with prefix 're-'
Location: .stash/research/
Daemon started (PID 12345)

Run 'stash onboard' to get CLAUDE.md snippet.
```

#### `stash drop`

Delete a stash and all its data.

```bash
stash drop <name> [--yes]
```

#### `stash info`

Show status of all stashes.

```bash
stash info [--json]
```

Output:
```
Stashes:
  research (re-)  100 records, 47 files, synced
  contacts (ct-)   25 records,  0 files, synced

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

This project uses **stash** for structured research data.
Run `stash prime` for current context.

**Quick reference:**
- `stash list --json` - List all records (structured)
- `stash add "value"` - Create record
- `stash set <id> <col> <val>` - Update field
- `stash file <id> <col> --content "..."` - Attach markdown
- `stash list --where "col = value"` - Filter records
- `stash column list` - See available columns

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

## Active Stashes

### research (prefix: re-)
Columns:
  - CompanyName: Official registered company name
  - Verified: true if company existence confirmed via official sources
  - Overview: Markdown file with company summary

Records: 100 total
  - 47 with Verified = true
  - 53 with Verified = false or empty
  - 32 with Overview attached

### contacts (prefix: ct-)
Columns:
  - Name: Full name
  - Email: Email address
  - Company: Company name (can reference research stash)

Records: 25 total

## Quick Commands
- Iterate: `stash list --json | jq -c '.[]'`
- Filter: `stash list --where "Verified = false" --json`
- Update: `stash set <id> <column> <value>`
```

---

### Schema Management

#### `stash column add`

Add one or more columns to a stash.

```bash
stash column add <name>... [--desc "description"] [--stash <name>]

# Examples
stash column add CompanyName
stash column add Verified Overview CEO
stash column add Revenue --desc "Annual revenue in USD"
```

#### `stash column list`

List columns with descriptions and statistics.

```bash
stash column list [--stash <name>] [--json]
```

Output (table):
```
Column       Description                              Populated  Empty
───────────────────────────────────────────────────────────────────────
CompanyName  Official registered company name         100        0
Verified     true if company existence confirmed       47       53
Overview     Markdown file with company summary        32       68
CEO          Current CEO full name                     28       72
```

Output (JSON):
```json
[
  {"name": "CompanyName", "desc": "Official registered company name", "populated": 100, "empty": 0},
  {"name": "Verified", "desc": "true if company existence confirmed", "populated": 47, "empty": 53}
]
```

#### `stash column describe`

Set or update a column description.

```bash
stash column describe <name> "description"

# Example
stash column describe CEO "Current CEO full name, as of last verification"
```

#### `stash column rename`

Rename a column.

```bash
stash column rename <old-name> <new-name>

# Example
stash column rename Company CompanyName
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
stash add "Microsoft"
stash add "Azure" --parent re-ex4j
stash add "Apple" --set CEO "Tim Cook" --set Founded 1976

# Output
re-ex4j

# Output (--json)
{"id": "re-ex4j", "CompanyName": "Microsoft"}
```

The primary value goes into the first column defined in the schema.

#### `stash set`

Update a field on a record.

```bash
stash set <id> <column> <value>

# Examples
stash set re-ex4j Verified true
stash set re-ex4j CEO "Satya Nadella"
stash set re-ex4j.1 Verified true

# Multiple columns
stash set re-ex4j --col Verified true --col CEO "Satya Nadella"
```

#### `stash file`

Create or attach a markdown file to a record.

```bash
stash file <id> <column> [--content "..."] [--from <path>]

# Create with inline content
stash file re-ex4j Overview --content "# Microsoft

Founded in 1975 by Bill Gates and Paul Allen.

## Products
- Windows
- Office
- Azure
"

# Copy from existing file
stash file re-ex4j Overview --from ./research/microsoft-notes.md

# Output
Created .stash/research/files/re-ex4j.md
Updated re-ex4j.Overview = "re-ex4j.md"
```

#### `stash show`

Display a record with all fields.

```bash
stash show <id> [--json] [--with-files]

# Examples
stash show re-ex4j
stash show re-ex4j --json
stash show re-ex4j --with-files  # Include file contents
```

Output (table):
```
Record: re-ex4j
Parent: (none)
Created: 2025-01-08 10:30:00
Updated: 2025-01-08 11:10:00

Fields:
  CompanyName: Microsoft
  Verified: true
  Overview: re-ex4j.md
  CEO: Satya Nadella

Children:
  re-ex4j.1  Azure
  re-ex4j.2  Microsoft 365
```

Output (JSON):
```json
{
  "id": "re-ex4j",
  "parent": null,
  "created": "2025-01-08T10:30:00Z",
  "updated": "2025-01-08T11:10:00Z",
  "fields": {
    "CompanyName": "Microsoft",
    "Verified": true,
    "Overview": "re-ex4j.md",
    "CEO": "Satya Nadella"
  },
  "children": ["re-ex4j.1", "re-ex4j.2"]
}
```

Output (JSON with `--with-files`):
```json
{
  "id": "re-ex4j",
  "fields": {
    "CompanyName": "Microsoft",
    "Overview": "re-ex4j.md"
  },
  "_files": {
    "Overview": "# Microsoft\n\nFounded in 1975..."
  }
}
```

#### `stash delete`

Delete a record.

```bash
stash delete <id> [--cascade] [--yes]

# Flags
--cascade    Also delete all children
--yes        Skip confirmation
```

---

### Querying

#### `stash list`

List records with optional filtering.

```bash
stash list [--where "..."] [--columns "..."] [--tree] [--json] [--stash <name>]

# Examples
stash list
stash list --where "Verified = false"
stash list --where "CEO IS NOT NULL"
stash list --columns "id,CompanyName,Verified"
stash list --tree
stash list --json
```

Output (table):
```
ID        CompanyName   Verified  Overview
────────────────────────────────────────────
re-ex4j   Microsoft     true      re-ex4j.md
re-8t5n   Apple         true      re-8t5n.md
re-k2m9   Acme Corp     false     -
```

Output (tree):
```
re-ex4j   Microsoft
├─ re-ex4j.1   Azure
│  └─ re-ex4j.1.1   Azure OpenAI
└─ re-ex4j.2   Microsoft 365
re-8t5n   Apple
└─ re-8t5n.1   Apple Services
re-k2m9   Acme Corp
```

Output (JSON):
```json
[
  {"id": "re-ex4j", "CompanyName": "Microsoft", "Verified": true, "Overview": "re-ex4j.md"},
  {"id": "re-8t5n", "CompanyName": "Apple", "Verified": true, "Overview": "re-8t5n.md"}
]
```

#### `stash children`

List direct children of a record.

```bash
stash children <id> [--json]

# Example
stash children re-ex4j
```

#### `stash query`

Execute raw SQL against the cache.

```bash
stash query "<sql>" [--json]

# Examples
stash query "SELECT CompanyName, CEO FROM research WHERE Verified = 'true'"
stash query "SELECT COUNT(*) as total FROM research"
stash query "SELECT * FROM research WHERE CompanyName LIKE '%soft%'"
```

---

### Import/Export

#### `stash import`

Import records from CSV.

```bash
stash import <file.csv> [--stash <name>] [--column <primary>] [--confirm] [--dry-run]

# Examples
stash import companies.csv
stash import companies.csv --column CompanyName
stash import companies.csv --dry-run
stash import companies.csv --confirm  # Skip interactive prompt
```

Workflow:
1. Parse CSV headers
2. Show column preview with sample data
3. Ask for confirmation (unless `--confirm`)
4. Create missing columns
5. Import records

Output:
```
Importing from companies.csv

Detected columns:
  CompanyName  (100 values, 0 empty) - exists
  Industry     (98 values, 2 empty)  - NEW
  Website      (95 values, 5 empty)  - NEW

This will:
  - Add columns: Industry, Website
  - Create 100 new records with prefix: re-

Proceed? [y/N]
```

#### `stash export`

Export records to CSV or JSON.

```bash
stash export <file> [--stash <name>] [--where "..."] [--format csv|json]

# Examples
stash export results.csv
stash export results.json --format json
stash export verified.csv --where "Verified = true"
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
  research: in-sync (100 records)
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
--deep    Full validation (slower)
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

Output:
```
Stash Doctor

✓ Daemon running (PID 12345)
✓ Cache database valid
✓ research: 100 records, in-sync
✓ contacts: 25 records, in-sync
⚠ research: 3 file references point to missing files
⚠ contacts: Column 'Email' has no description

Issues found: 2 warnings, 0 errors

Run 'stash doctor --fix' to repair.
```

#### `stash repair`

Emergency repair for corrupted data.

```bash
stash repair [--dry-run] [--source jsonl|db] [--clean-orphans]

# Flags
--dry-run         Preview repairs without making changes
--source jsonl    Force rebuild from JSONL (recommended)
--source db       Force rebuild JSONL from DB
--clean-orphans   Remove orphaned files in files/
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

## 5. Daemon Design

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

## 6. Go Implementation

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
    record.go            # Record type, ID generation
    column.go            # Column type, schema
    stash.go             # Stash type
  daemon/
    daemon.go            # Daemon process management
    watcher.go           # File watching
  cli/
    root.go              # Root command
    init.go              # stash init
    add.go               # stash add
    set.go               # stash set
    list.go              # stash list
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
// model/stash.go
type Stash struct {
    Name    string    `json:"name"`
    Prefix  string    `json:"prefix"`
    Created time.Time `json:"created"`
    Columns []Column  `json:"columns"`
}

// model/column.go
type Column struct {
    Name  string    `json:"name"`
    Desc  string    `json:"desc"`
    Added time.Time `json:"added"`
}

// model/record.go
type Record struct {
    ID        string                 `json:"_id"`
    ParentID  string                 `json:"_parent,omitempty"`
    Timestamp time.Time              `json:"_ts"`
    Operation string                 `json:"_op"`
    Fields    map[string]interface{} `json:"-"` // Flattened in JSON
}

// storage/jsonl.go
type JSONLStore struct {
    path string
}

func (s *JSONLStore) Append(record Record) error
func (s *JSONLStore) ReadAll() ([]Record, error)
func (s *JSONLStore) Compact() error  // Merge updates, remove deletes

// storage/sqlite.go
type SQLiteCache struct {
    db *sql.DB
}

func (c *SQLiteCache) EnsureTable(stash Stash) error
func (c *SQLiteCache) EnsureColumn(table, column string) error
func (c *SQLiteCache) Upsert(table string, record Record) error
func (c *SQLiteCache) Query(sql string) ([]map[string]interface{}, error)
```

---

## 7. Error Handling

### Error Types

```go
var (
    ErrStashNotFound    = errors.New("stash not found")
    ErrStashExists      = errors.New("stash already exists")
    ErrRecordNotFound   = errors.New("record not found")
    ErrColumnNotFound   = errors.New("column not found")
    ErrColumnExists     = errors.New("column already exists")
    ErrInvalidID        = errors.New("invalid record ID")
    ErrParentNotFound   = errors.New("parent record not found")
    ErrDaemonNotRunning = errors.New("daemon not running")
    ErrSyncConflict     = errors.New("sync conflict detected")
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
```

---

## 8. Configuration

### Environment Variables

```bash
STASH_DIR=.stash           # Stash directory location
STASH_DEFAULT=research     # Default stash for commands
STASH_NO_DAEMON=1          # Disable daemon auto-start
STASH_LOG_LEVEL=debug      # Log verbosity
```

### Global Config (optional)

`~/.config/stash/config.json`:
```json
{
  "default_stash": "research",
  "daemon_auto_start": true,
  "log_level": "info"
}
```

---

## 9. Future Considerations (v2+)

- **Column types**: Optional type hints (string, int, bool, file, date)
- **Validation**: Column constraints (required, unique, regex)
- **Relationships**: Cross-stash references
- **Hooks**: Pre/post operation hooks
- **Encryption**: Encrypted columns for sensitive data
- **Remote sync**: Git-based sync like beads
- **Web UI**: Simple browser interface for viewing data

---

## Appendix A: JSONL Format Examples

### Create Record
```json
{"_id":"re-ex4j","_ts":"2025-01-08T10:30:00Z","_op":"create","CompanyName":"Microsoft"}
```

### Update Field
```json
{"_id":"re-ex4j","_ts":"2025-01-08T11:00:00Z","_op":"update","Verified":true,"CEO":"Satya Nadella"}
```

### Create Child Record
```json
{"_id":"re-ex4j.1","_ts":"2025-01-08T11:05:00Z","_op":"create","_parent":"re-ex4j","CompanyName":"Azure"}
```

### Delete Record
```json
{"_id":"re-ex4j","_ts":"2025-01-08T12:00:00Z","_op":"delete","_deleted":true}
```

---

## Appendix B: SQLite Query Examples

```sql
-- All records in a stash
SELECT * FROM research WHERE deleted_at IS NULL;

-- Filter by field
SELECT id, CompanyName FROM research WHERE Verified = 'true';

-- Count by field value
SELECT Verified, COUNT(*) as count FROM research GROUP BY Verified;

-- Hierarchy: find all descendants
WITH RECURSIVE descendants AS (
    SELECT id, parent_id, CompanyName, 0 as depth
    FROM research WHERE id = 're-ex4j'
    UNION ALL
    SELECT r.id, r.parent_id, r.CompanyName, d.depth + 1
    FROM research r
    JOIN descendants d ON r.parent_id = d.id
)
SELECT * FROM descendants;

-- Records with missing files
SELECT id, Overview FROM research
WHERE Overview IS NOT NULL
AND Overview NOT IN (SELECT filename FROM _files);
```
