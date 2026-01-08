# Stash Architecture

> **Last verified**: 2026-01-08 | **Commit**: b423181

---

## System Overview

```yaml
system:
  name: stash
  description: Record-centric structured data store for AI agents
  type: cli
  language: Go 1.22+

status:
  phase: development
  version: 2.2.0
```

---

## Components

```yaml
components:
  cli:
    path: cmd/stash/
    purpose: Command-line interface entry point
    owns: [main, cobra commands]
    depends_on: [internal/cli]

  cli_commands:
    path: internal/cli/
    purpose: CLI command implementations
    owns: [init, add, set, get, list, drop, column, query]
    depends_on: [storage, model, output, config, context]

  storage:
    path: internal/storage/
    purpose: Dual storage layer (JSONL + SQLite)
    owns: [JSONL operations, SQLite cache, file attachments]
    depends_on: [model]

  model:
    path: internal/model/
    purpose: Domain models and business logic
    owns: [Record, Column, Stash types, validation]
    depends_on: []

  output:
    path: internal/output/
    purpose: Output formatting (text, JSON)
    owns: [formatters, printers]
    depends_on: [model]

  config:
    path: internal/config/
    purpose: Configuration loading and validation
    owns: [config.json handling, defaults]
    depends_on: []

  context:
    path: internal/context/
    purpose: Actor and branch context detection
    owns: [git integration, user detection]
    depends_on: []

  daemon:
    path: internal/daemon/
    purpose: Background file watcher for JSONL->SQLite sync
    owns: [file watching, sync operations]
    depends_on: [storage]
```

---

## Principles

```yaml
principles:
  record_centric:
    rule: Records are primary; files are optional attachments
    why: Matches how AI agents think about structured data
    violation: Making file operations the default path

  agent_first:
    rule: Commands map to natural language instructions
    why: AI agents can invoke commands conversationally
    violation: Commands requiring complex flags for basic operations

  dual_storage:
    rule: JSONL is source of truth, SQLite is query cache
    why: Human-readable storage + fast queries
    violation: Writing directly to SQLite, skipping JSONL

  fluid_schema:
    rule: Add columns anytime without migrations
    why: Agents shouldn't need schema planning upfront
    violation: Requiring schema definition before adding records

  ucdd:
    rule: All features specified in usecases/*.yaml before implementation
    why: Prevents scope creep, enables test-driven development
    violation: Implementing features not in acceptance criteria
```

---

## Boundaries

```yaml
boundaries:
  in_scope:
    - Structured data storage (records, columns)
    - Hierarchical records (parent-child)
    - JSONL + SQLite dual storage
    - CLI interface for agents
    - JSON output for programmatic access
    - Actor and branch tracking
    - File attachments per record

  out_of_scope:
    - Task/issue tracking (use Beads instead)
    - Real-time collaboration
    - Remote sync (local only)
    - Web interface
    - Multi-user access control

  must_not:
    - Write to SQLite without updating JSONL first
    - Break JSONL format (must stay human-readable)
    - Require internet access
    - Store secrets in stash data
```

---

## Data Flow

```yaml
data_flow:
  write_operation:
    1_command: User runs `stash add` or `stash set`
    2_validate: CLI validates input, generates ID if needed
    3_context: Context module gets actor and branch
    4_jsonl: Storage appends record to records.jsonl
    5_sqlite: Storage updates SQLite cache table
    6_output: CLI outputs result (text or JSON)

  read_operation:
    1_command: User runs `stash get` or `stash list`
    2_query: Storage queries SQLite cache
    3_format: Output module formats results
    4_output: CLI outputs to stdout

  daemon_sync:
    1_watch: Daemon watches records.jsonl for changes
    2_detect: File modification detected
    3_rebuild: Daemon rebuilds SQLite cache from JSONL
    4_ready: Cache is consistent with JSONL
```

---

## Documentation Map

```yaml
docs:
  spec:
    path: specs/spec.md
    covers: Full technical specification
    read_when: Understanding system design

  use_cases:
    path: usecases/
    covers: All features with acceptance criteria
    read_when: Implementing or testing features
    files:
      - stash.yaml (UC-ST-*): Stash lifecycle
      - columns.yaml (UC-COL-*): Column management
      - records.yaml (UC-REC-*): Record operations
      - query.yaml (UC-QRY-*): Querying
      - import.yaml (UC-IMP-*): Import/export
      - sync.yaml (UC-SYN-*, UC-DMN-*): Sync and daemon

  tests:
    path: tests/
    covers: CLI integration tests
    read_when: Running or adding tests
```

---

## Technology Stack

```yaml
stack:
  language: Go 1.22+
  cli_framework: cobra
  database: SQLite (via go-sqlite3)
  file_watching: fsnotify
  testing: testify (assert, require)
  storage_format: JSONL (line-delimited JSON)
```

---

## Storage Layout

```yaml
storage:
  stash_directory: .stash/<stash-name>/
  files:
    - config.json: Stash metadata (name, prefix, created_at)
    - records.jsonl: Source of truth for all records
    - cache.db: SQLite cache (derived from JSONL)
    - files/: Attached files per record

  record_format:
    system_fields:
      - _id: Unique identifier (<prefix>-<hash>)
      - _hash: SHA-256 of user data
      - _created_at: UTC timestamp
      - _created_by: Actor name
      - _updated_at: Last modification time
      - _updated_by: Last modifier
      - _branch: Git branch (if available)
      - _parent: Parent record ID (optional)
      - _deleted_at: Soft-delete timestamp
      - _deleted_by: Soft-delete actor
    user_fields: Dynamic, defined by columns
```

---

## Maintenance

```yaml
maintenance:
  review_frequency: monthly

  update_triggers:
    - New component added
    - Principle changed
    - Boundary modified
    - New use case file created

  ownership: All developers (update when you change architecture)
```
