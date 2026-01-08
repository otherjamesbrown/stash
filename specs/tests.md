# Stash: Test Suite Design

This document defines the test strategy using **Use Case Driven Development (UCDD)**.

> **UCDD** is a specification-first testing approach where requirements are captured in structured YAML files before implementation. Each acceptance criterion becomes a test, preventing scope creep and ensuring testable deliverables.
>
> See: [UCDD Methodology](https://github.com/otherjamesbrown/dev-setup/tree/main/templates/testing)

---

## Directory Structure

```
stash/
├── usecases/                       # Use case specifications (YAML)
│   ├── SCHEMA.md                   # Schema documentation
│   ├── stash.yaml                  # Stash management (init, drop, info)
│   ├── records.yaml                # Record operations (add, set, delete)
│   ├── columns.yaml                # Schema management
│   ├── query.yaml                  # Querying (list, query, history)
│   ├── sync.yaml                   # Sync and maintenance
│   └── import.yaml                 # Import/export operations
│
├── tests/
│   ├── usecases/                   # UCDD tests (map to use cases)
│   │   ├── stash_test.go           # Tests for UC-ST-*
│   │   ├── records_test.go         # Tests for UC-REC-*
│   │   ├── columns_test.go         # Tests for UC-COL-*
│   │   ├── query_test.go           # Tests for UC-QRY-*
│   │   ├── sync_test.go            # Tests for UC-SYN-*
│   │   └── import_test.go          # Tests for UC-IMP-*
│   │
│   ├── unit/                       # Unit tests (internal functions)
│   │   ├── model/
│   │   ├── storage/
│   │   └── output/
│   │
│   └── fixtures/                   # Test data files
│       ├── products.csv
│       └── corrupted.jsonl
│
└── specs/
    ├── spec.md                     # Technical specification
    ├── use-cases.md                # Workflow scenarios
    └── tests.md                    # This file
```

---

## Use Case Prefixes

| Prefix | Feature Area | File |
|--------|--------------|------|
| UC-ST- | Stash management | `stash.yaml` |
| UC-REC- | Record operations | `records.yaml` |
| UC-COL- | Column/schema management | `columns.yaml` |
| UC-QRY- | Querying and listing | `query.yaml` |
| UC-SYN- | Sync, daemon, maintenance | `sync.yaml` |
| UC-IMP- | Import/export | `import.yaml` |

---

## Use Case Files

### usecases/stash.yaml

```yaml
feature: Stash Management
description: |
  Core stash lifecycle operations: creating, removing, and inspecting stashes.
  A stash is a named collection of records with a unique ID prefix.

usecases:
  - id: UC-ST-001
    title: Initialize Stash
    interface: cli
    status: active

    description: |
      User wants to create a new stash to store structured data.
      The stash is initialized with a name, prefix, and empty schema.

    actor: User

    preconditions:
      - .stash directory may or may not exist
      - No stash with same name exists

    acceptance_criteria:
      - id: AC-01
        criterion: Create stash with required fields
        given: No stash named "inventory" exists
        when: User runs `stash init inventory --prefix inv-`
        then:
          - Directory .stash/inventory/ is created
          - config.json contains name, prefix, created_at, created_by
          - Empty records.jsonl is created
          - files/ subdirectory is created
          - Daemon is started (unless --no-daemon)
          - Exit code is 0

      - id: AC-02
        criterion: Reject duplicate stash name
        given: Stash "inventory" already exists
        when: User runs `stash init inventory --prefix inv-`
        then:
          - Command fails with exit code 1
          - Error message indicates stash exists
          - No files are modified

      - id: AC-03
        criterion: Reject invalid prefix
        given: No stash named "test" exists
        when: User runs `stash init test --prefix x` (too short)
        then:
          - Command fails with exit code 2
          - Error message explains prefix requirements (2-4 chars)
          - No files are created

      - id: AC-04
        criterion: Skip daemon with flag
        given: No stash named "test" exists
        when: User runs `stash init test --prefix ts- --no-daemon`
        then:
          - Stash is created successfully
          - Daemon is NOT started
          - Exit code is 0

      - id: AC-05
        criterion: Capture actor and branch
        given: User is "alice" on branch "main"
        when: User runs `stash init inventory --prefix inv-`
        then:
          - config.json created_by is "alice"
          - First operation records _branch as "main"

    in_scope:
      - Creating .stash directory structure
      - Creating config.json with metadata
      - Creating empty records.jsonl
      - Starting daemon (default behavior)
      - Capturing actor and branch context

    out_of_scope:
      - Adding columns (see UC-COL-001)
      - Adding records (see UC-REC-001)
      - Modifying existing stashes

    must_not:
      - Overwrite existing stash
      - Start daemon if --no-daemon specified
      - Create stash with invalid prefix

  - id: UC-ST-002
    title: Drop Stash
    interface: cli
    status: active

    description: |
      User wants to permanently delete a stash and all its data.
      This is destructive and cannot be undone.

    actor: User

    preconditions:
      - Stash exists

    acceptance_criteria:
      - id: AC-01
        criterion: Drop stash with confirmation
        given: Stash "inventory" exists with records
        when: User runs `stash drop inventory` and confirms
        then:
          - Directory .stash/inventory/ is deleted
          - SQLite table is dropped from cache.db
          - Exit code is 0

      - id: AC-02
        criterion: Skip confirmation with --yes
        given: Stash "inventory" exists
        when: User runs `stash drop inventory --yes`
        then:
          - Stash is deleted without prompting
          - Exit code is 0

      - id: AC-03
        criterion: Reject non-existent stash
        given: No stash named "fake" exists
        when: User runs `stash drop fake --yes`
        then:
          - Command fails with exit code 3
          - Error message indicates stash not found

    in_scope:
      - Deleting stash directory
      - Removing from SQLite cache
      - Confirmation prompt

    out_of_scope:
      - Soft-delete (stashes are always hard-deleted)
      - Backup before delete

    must_not:
      - Delete without confirmation (unless --yes)
      - Leave orphaned SQLite tables

  - id: UC-ST-003
    title: Show Stash Info
    interface: cli
    status: active

    description: |
      User wants to see status of all stashes including record counts,
      daemon status, and current context (actor, branch).

    actor: User

    preconditions:
      - At least one stash exists (or shows empty message)

    acceptance_criteria:
      - id: AC-01
        criterion: Show all stashes with stats
        given: Stashes "inventory" (50 records) and "contacts" (25 records) exist
        when: User runs `stash info`
        then:
          - Output lists both stashes with prefixes
          - Record counts are shown
          - Deleted record counts are shown
          - File counts are shown
          - Daemon status is shown
          - Current actor and branch are shown

      - id: AC-02
        criterion: JSON output format
        given: Stash "inventory" exists
        when: User runs `stash info --json`
        then:
          - Output is valid JSON
          - Contains stashes array with stats
          - Contains daemon status object
          - Contains context (actor, branch)

    in_scope:
      - Listing all stashes
      - Record and file counts
      - Daemon status
      - Actor and branch context

    out_of_scope:
      - Detailed record listing (see UC-QRY-001)
      - Column information (see UC-COL-002)

    must_not:
      - Show deleted records in main count (show separately)
```

### usecases/records.yaml

```yaml
feature: Record Operations
description: |
  CRUD operations for records within a stash. Records have system fields
  (_id, _hash, _created_at, etc.) and user-defined columns.

dependencies:
  - usecases/stash.yaml
  - usecases/columns.yaml

usecases:
  - id: UC-REC-001
    title: Add Record
    interface: cli
    status: active

    description: |
      User wants to create a new record in a stash. The record gets
      a unique ID, hash, and audit fields automatically.

    actor: User

    preconditions:
      - Stash exists
      - At least one column is defined

    depends_on:
      - UC-ST-001
      - UC-COL-001

    acceptance_criteria:
      - id: AC-01
        criterion: Add record with primary value
        given: Stash "inventory" exists with column "Name"
        when: User runs `stash add "Laptop"`
        then:
          - Record is created with unique ID (inv-xxxx)
          - Name field is set to "Laptop"
          - _hash is calculated from user fields
          - _created_at and _updated_at are set to now
          - _created_by and _updated_by are set to current actor
          - _branch is set to current git branch
          - ID is output to stdout
          - Exit code is 0

      - id: AC-02
        criterion: Add record with additional fields
        given: Stash "inventory" exists with columns "Name", "Price", "Category"
        when: User runs `stash add "Laptop" --set Price 999 --set Category "electronics"`
        then:
          - Record is created with all three fields set
          - ID is output to stdout
          - Exit code is 0

      - id: AC-03
        criterion: Add child record
        given: Record inv-ex4j exists in "inventory"
        when: User runs `stash add "Charger" --parent inv-ex4j`
        then:
          - Child record is created with ID inv-ex4j.1
          - _parent field is set to inv-ex4j
          - Exit code is 0

      - id: AC-04
        criterion: Reject invalid parent
        given: No record inv-fake exists
        when: User runs `stash add "Charger" --parent inv-fake`
        then:
          - Command fails with exit code 4
          - Error message indicates parent not found
          - No record is created

      - id: AC-05
        criterion: JSON output format
        given: Stash "inventory" exists with column "Name"
        when: User runs `stash add "Laptop" --json`
        then:
          - Output is valid JSON object
          - Contains _id, _hash, _created_by, _branch, Name

    in_scope:
      - Creating record with ID
      - Setting primary value to first column
      - Setting additional fields via --set
      - Creating child records with --parent
      - Calculating hash
      - Capturing audit fields

    out_of_scope:
      - Bulk import (see UC-IMP-001)
      - Updating existing records (see UC-REC-002)

    must_not:
      - Create record without at least one column defined
      - Allow duplicate IDs
      - Create child without valid parent

  - id: UC-REC-002
    title: Update Record Field
    interface: cli
    status: active

    description: |
      User wants to update one or more fields on an existing record.
      This updates the hash and audit trail.

    actor: User

    preconditions:
      - Record exists and is not deleted
      - Column exists in schema

    depends_on:
      - UC-REC-001

    acceptance_criteria:
      - id: AC-01
        criterion: Update single field
        given: Record inv-ex4j exists with Name="Laptop"
        when: User runs `stash set inv-ex4j Price 1299`
        then:
          - Price field is set to 1299
          - _hash is recalculated
          - _updated_at is set to now
          - _updated_by is set to current actor
          - Exit code is 0

      - id: AC-02
        criterion: Update multiple fields
        given: Record inv-ex4j exists
        when: User runs `stash set inv-ex4j --col Price 1299 --col Stock 50`
        then:
          - Both fields are updated
          - Single JSONL entry is appended (not two)
          - Exit code is 0

      - id: AC-03
        criterion: Reject non-existent record
        given: No record inv-fake exists
        when: User runs `stash set inv-fake Price 100`
        then:
          - Command fails with exit code 4
          - Error message indicates record not found

      - id: AC-04
        criterion: Reject non-existent column
        given: Record inv-ex4j exists, no column "FakeCol"
        when: User runs `stash set inv-ex4j FakeCol "value"`
        then:
          - Command fails with exit code 1
          - Error message indicates column not found
          - Record is not modified

      - id: AC-05
        criterion: Reject update to deleted record
        given: Record inv-ex4j is soft-deleted
        when: User runs `stash set inv-ex4j Price 100`
        then:
          - Command fails with appropriate error
          - Suggests using `stash restore` first

    in_scope:
      - Updating single field
      - Updating multiple fields atomically
      - Recalculating hash
      - Updating audit trail

    out_of_scope:
      - Creating new records (see UC-REC-001)
      - Adding new columns (see UC-COL-001)

    must_not:
      - Allow update to non-existent column
      - Allow update to deleted record
      - Create multiple JSONL entries for single set command

  - id: UC-REC-003
    title: Delete Record (Soft)
    interface: cli
    status: active

    description: |
      User wants to soft-delete a record. The record remains in the
      database but is marked as deleted and excluded from normal queries.

    actor: User

    preconditions:
      - Record exists and is not already deleted

    depends_on:
      - UC-REC-001

    acceptance_criteria:
      - id: AC-01
        criterion: Soft-delete record
        given: Record inv-ex4j exists and is active
        when: User runs `stash delete inv-ex4j`
        then:
          - _deleted_at is set to now
          - _deleted_by is set to current actor
          - Record is excluded from `stash list`
          - Exit code is 0

      - id: AC-02
        criterion: Delete with cascade
        given: Record inv-ex4j has children inv-ex4j.1 and inv-ex4j.2
        when: User runs `stash delete inv-ex4j --cascade`
        then:
          - Parent and all children are soft-deleted
          - Exit code is 0

      - id: AC-03
        criterion: Reject delete without cascade when children exist
        given: Record inv-ex4j has children
        when: User runs `stash delete inv-ex4j` (no --cascade)
        then:
          - Command fails with appropriate error
          - Suggests using --cascade
          - No records are deleted

      - id: AC-04
        criterion: Skip confirmation with --yes
        given: Record inv-ex4j exists
        when: User runs `stash delete inv-ex4j --yes`
        then:
          - Record is deleted without prompting
          - Exit code is 0

    in_scope:
      - Setting _deleted_at and _deleted_by
      - Cascade delete of children
      - Confirmation prompt

    out_of_scope:
      - Permanent deletion (see UC-REC-005)
      - Restoring deleted records (see UC-REC-004)

    must_not:
      - Permanently remove data
      - Delete children without --cascade flag

  - id: UC-REC-004
    title: Restore Deleted Record
    interface: cli
    status: active

    description: |
      User wants to restore a soft-deleted record, making it active again.

    actor: User

    preconditions:
      - Record exists and is soft-deleted

    depends_on:
      - UC-REC-003

    acceptance_criteria:
      - id: AC-01
        criterion: Restore deleted record
        given: Record inv-ex4j is soft-deleted
        when: User runs `stash restore inv-ex4j`
        then:
          - _deleted_at is set to null
          - _deleted_by is set to null
          - _updated_at and _updated_by are updated
          - Record appears in `stash list`
          - Exit code is 0

      - id: AC-02
        criterion: Restore with cascade
        given: Record inv-ex4j and children are soft-deleted
        when: User runs `stash restore inv-ex4j --cascade`
        then:
          - Parent and all deleted children are restored
          - Exit code is 0

      - id: AC-03
        criterion: Reject restore of active record
        given: Record inv-ex4j is active (not deleted)
        when: User runs `stash restore inv-ex4j`
        then:
          - Command fails or is no-op
          - Appropriate message shown

    in_scope:
      - Clearing _deleted_at and _deleted_by
      - Cascade restore of children
      - Updating audit trail

    out_of_scope:
      - Restoring purged records (impossible)

    must_not:
      - Restore records that were permanently purged

  - id: UC-REC-005
    title: Purge Deleted Records
    interface: cli
    status: active

    description: |
      User wants to permanently remove soft-deleted records.
      This is irreversible and removes data from JSONL.

    actor: User

    preconditions:
      - Soft-deleted records exist

    depends_on:
      - UC-REC-003

    acceptance_criteria:
      - id: AC-01
        criterion: Purge by age
        given: Records deleted more than 30 days ago exist
        when: User runs `stash purge --before 30d --yes`
        then:
          - Records deleted > 30 days ago are permanently removed
          - JSONL entries are removed
          - Associated files in files/ are deleted
          - Exit code is 0

      - id: AC-02
        criterion: Purge specific record
        given: Record inv-ex4j is soft-deleted
        when: User runs `stash purge --id inv-ex4j --yes`
        then:
          - Only that record is permanently removed
          - Exit code is 0

      - id: AC-03
        criterion: Dry run preview
        given: Soft-deleted records exist
        when: User runs `stash purge --before 30d --dry-run`
        then:
          - Output lists records that would be purged
          - No records are actually removed
          - Exit code is 0

      - id: AC-04
        criterion: Require confirmation
        given: Soft-deleted records exist
        when: User runs `stash purge --all` (no --yes)
        then:
          - Confirmation prompt is shown
          - No action without confirmation

    in_scope:
      - Permanently removing JSONL entries
      - Removing associated files
      - Filtering by age or ID
      - Dry run preview

    out_of_scope:
      - Recovering purged records (impossible)

    must_not:
      - Purge without confirmation (unless --yes)
      - Purge active (non-deleted) records
```

### usecases/columns.yaml

```yaml
feature: Column Management
description: |
  Schema operations for managing columns within a stash. Columns define
  the structure of records and include descriptions to help agents.

dependencies:
  - usecases/stash.yaml

usecases:
  - id: UC-COL-001
    title: Add Column
    interface: cli
    status: active

    description: |
      User wants to add one or more columns to a stash schema.
      Columns can have descriptions to help agents understand their purpose.

    actor: User

    preconditions:
      - Stash exists

    depends_on:
      - UC-ST-001

    acceptance_criteria:
      - id: AC-01
        criterion: Add single column
        given: Stash "inventory" exists with no columns
        when: User runs `stash column add Name`
        then:
          - Column "Name" is added to config.json
          - Column has added timestamp and added_by
          - SQLite table is altered to add column
          - Exit code is 0

      - id: AC-02
        criterion: Add multiple columns
        given: Stash "inventory" exists
        when: User runs `stash column add Name Price Category`
        then:
          - All three columns are added
          - Exit code is 0

      - id: AC-03
        criterion: Add column with description
        given: Stash "inventory" exists
        when: User runs `stash column add Price --desc "Price in USD"`
        then:
          - Column "Price" is added with description
          - Description appears in `stash column list`

      - id: AC-04
        criterion: Reject duplicate column
        given: Column "Name" already exists
        when: User runs `stash column add Name`
        then:
          - Command fails with appropriate error
          - No duplicate column created

      - id: AC-05
        criterion: Reject reserved names
        given: Stash "inventory" exists
        when: User runs `stash column add _id`
        then:
          - Command fails with exit code 2
          - Error explains reserved column names

    in_scope:
      - Adding columns to config.json
      - Adding columns to SQLite via ALTER TABLE
      - Setting descriptions
      - Tracking who added column

    out_of_scope:
      - Column types (future v2)
      - Column validation rules (future v2)

    must_not:
      - Allow duplicate column names
      - Allow reserved names (_id, _hash, _created_at, etc.)
      - Allow invalid characters in column names

  - id: UC-COL-002
    title: List Columns
    interface: cli
    status: active

    description: |
      User wants to see all columns in a stash with their descriptions
      and population statistics.

    actor: User

    preconditions:
      - Stash exists

    acceptance_criteria:
      - id: AC-01
        criterion: List columns with stats
        given: Stash has columns Name (100 populated), Price (75 populated)
        when: User runs `stash column list`
        then:
          - Output shows column names
          - Output shows descriptions
          - Output shows populated count
          - Output shows empty count

      - id: AC-02
        criterion: JSON output format
        given: Stash has columns
        when: User runs `stash column list --json`
        then:
          - Output is valid JSON array
          - Each entry has name, desc, populated, empty

    in_scope:
      - Listing column names and descriptions
      - Population statistics
      - JSON output format

    out_of_scope:
      - Modifying columns (see UC-COL-003, UC-COL-004)

    must_not:
      - Include system columns in list

  - id: UC-COL-003
    title: Rename Column
    interface: cli
    status: active

    description: |
      User wants to rename a column. Data is preserved.

    actor: User

    preconditions:
      - Column exists
      - New name doesn't exist

    acceptance_criteria:
      - id: AC-01
        criterion: Rename column
        given: Column "Cost" exists with data
        when: User runs `stash column rename Cost Price`
        then:
          - Column is renamed in config.json
          - Column is renamed in SQLite
          - Existing data is preserved
          - Exit code is 0

      - id: AC-02
        criterion: Reject rename to existing name
        given: Columns "Name" and "Title" both exist
        when: User runs `stash column rename Title Name`
        then:
          - Command fails with appropriate error
          - No changes made

    in_scope:
      - Renaming in config.json
      - Renaming in SQLite
      - Preserving data

    out_of_scope:
      - Merging columns
      - Renaming system columns

    must_not:
      - Lose data during rename
      - Allow rename to existing column name
```

### usecases/query.yaml

```yaml
feature: Querying
description: |
  Operations for listing, filtering, and querying records.

dependencies:
  - usecases/records.yaml

usecases:
  - id: UC-QRY-001
    title: List Records
    interface: cli
    status: active

    description: |
      User wants to list records with optional filtering and output formats.

    actor: User

    preconditions:
      - Stash exists

    acceptance_criteria:
      - id: AC-01
        criterion: List all active records
        given: Stash has 100 records (5 deleted)
        when: User runs `stash list`
        then:
          - 95 active records are shown
          - Deleted records are excluded
          - Exit code is 0

      - id: AC-02
        criterion: Filter with WHERE clause
        given: Stash has records with various Categories
        when: User runs `stash list --where "Category = 'electronics'"`
        then:
          - Only matching records are shown
          - Exit code is 0

      - id: AC-03
        criterion: Show as tree
        given: Stash has hierarchical records
        when: User runs `stash list --tree`
        then:
          - Records are shown in tree format
          - Children indented under parents
          - Exit code is 0

      - id: AC-04
        criterion: JSON output
        given: Stash has records
        when: User runs `stash list --json`
        then:
          - Output is valid JSON array
          - Each record includes _id, _hash, and user fields

      - id: AC-05
        criterion: Show deleted records
        given: Stash has 5 deleted records
        when: User runs `stash list --deleted`
        then:
          - Only deleted records are shown
          - Exit code is 0

    in_scope:
      - Listing active records
      - Filtering with WHERE clause
      - Tree view for hierarchy
      - JSON output format
      - Showing deleted records

    out_of_scope:
      - Complex SQL queries (see UC-QRY-002)
      - Pagination (future)

    must_not:
      - Show deleted records by default
      - Include system columns unless requested

  - id: UC-QRY-002
    title: Raw SQL Query
    interface: cli
    status: active

    description: |
      User wants to run arbitrary SQL against the SQLite cache.

    actor: User

    preconditions:
      - Stash exists with data in SQLite cache

    acceptance_criteria:
      - id: AC-01
        criterion: Execute SELECT query
        given: Stash "inventory" has records
        when: User runs `stash query "SELECT Name, Price FROM inventory WHERE Price > 100"`
        then:
          - Query results are displayed
          - Exit code is 0

      - id: AC-02
        criterion: Reject non-SELECT queries
        given: Stash exists
        when: User runs `stash query "DELETE FROM inventory"`
        then:
          - Command fails with appropriate error
          - No data is modified

    in_scope:
      - SELECT queries
      - JSON output format

    out_of_scope:
      - INSERT/UPDATE/DELETE (use stash commands)

    must_not:
      - Allow data modification via SQL
      - Execute queries against JSONL (cache only)

  - id: UC-QRY-003
    title: Show Record History
    interface: cli
    status: active

    description: |
      User wants to see the change history for a stash or specific record.

    actor: User

    preconditions:
      - Stash exists with records

    acceptance_criteria:
      - id: AC-01
        criterion: Show all recent changes
        given: Stash has records with multiple changes
        when: User runs `stash history`
        then:
          - Recent operations are listed
          - Shows timestamp, operation, ID, actor, branch
          - Exit code is 0

      - id: AC-02
        criterion: Show history for specific record
        given: Record inv-ex4j has been updated multiple times
        when: User runs `stash history inv-ex4j`
        then:
          - Only changes to that record are shown
          - Includes all operations (create, updates)

      - id: AC-03
        criterion: Filter by actor
        given: Changes by multiple actors exist
        when: User runs `stash history --by alice`
        then:
          - Only changes by alice are shown

      - id: AC-04
        criterion: Filter by time
        given: Changes over past week exist
        when: User runs `stash history --since 24h`
        then:
          - Only changes in last 24 hours shown

    in_scope:
      - Listing change history
      - Filtering by record, actor, time
      - JSON output format

    out_of_scope:
      - Reverting changes (future)
      - Diff view (future)

    must_not:
      - Show purged record history (it's gone)
```

---

## Test Implementation Pattern

Tests map directly to use cases with subtests for each acceptance criterion.

### Example: tests/usecases/stash_test.go

```go
package usecases_test

import (
    "testing"
    "os"
    "path/filepath"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestUC_ST_001_InitializeStash tests UC-ST-001
// See: usecases/stash.yaml
func TestUC_ST_001_InitializeStash(t *testing.T) {
    t.Run("AC-01: create stash with required fields", func(t *testing.T) {
        // Given: No stash named "inventory" exists
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)

        // When: User runs `stash init inventory --prefix inv-`
        stdout, stderr, err := runStash("init", "inventory", "--prefix", "inv-")

        // Then: Directory .stash/inventory/ is created
        require.NoError(t, err, "stderr: %s", stderr)
        assert.DirExists(t, ".stash/inventory")

        // Then: config.json contains name, prefix, created_at, created_by
        config := readConfig(t, ".stash/inventory/config.json")
        assert.Equal(t, "inventory", config.Name)
        assert.Equal(t, "inv-", config.Prefix)
        assert.NotEmpty(t, config.CreatedAt)
        assert.NotEmpty(t, config.CreatedBy)

        // Then: Empty records.jsonl is created
        assert.FileExists(t, ".stash/inventory/records.jsonl")

        // Then: files/ subdirectory is created
        assert.DirExists(t, ".stash/inventory/files")

        // Then: Exit code is 0
        assert.Contains(t, stdout, "Created stash")
    })

    t.Run("AC-02: reject duplicate stash name", func(t *testing.T) {
        // Given: Stash "inventory" already exists
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)
        runStash("init", "inventory", "--prefix", "inv-")

        // When: User runs `stash init inventory --prefix inv-`
        _, stderr, err := runStash("init", "inventory", "--prefix", "inv-")

        // Then: Command fails with exit code 1
        require.Error(t, err)

        // Then: Error message indicates stash exists
        assert.Contains(t, stderr, "already exists")
    })

    t.Run("AC-03: reject invalid prefix", func(t *testing.T) {
        // Given: No stash named "test" exists
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)

        // When: User runs `stash init test --prefix x` (too short)
        _, stderr, err := runStash("init", "test", "--prefix", "x")

        // Then: Command fails with exit code 2
        require.Error(t, err)

        // Then: Error message explains prefix requirements
        assert.Contains(t, stderr, "2-4")

        // Then: No files are created
        assert.NoDirExists(t, ".stash/test")
    })

    t.Run("AC-04: skip daemon with flag", func(t *testing.T) {
        // Given: No stash named "test" exists
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)

        // When: User runs `stash init test --prefix ts- --no-daemon`
        _, _, err := runStash("init", "test", "--prefix", "ts-", "--no-daemon")

        // Then: Stash is created successfully
        require.NoError(t, err)
        assert.DirExists(t, ".stash/test")

        // Then: Daemon is NOT started
        assert.NoFileExists(t, ".stash/daemon.pid")
    })
}

// TestUC_ST_001_MustNot tests anti-requirements for UC-ST-001
func TestUC_ST_001_InitializeStash_MustNot(t *testing.T) {
    t.Run("must not overwrite existing stash", func(t *testing.T) {
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)

        // Create initial stash
        runStash("init", "inventory", "--prefix", "inv-")

        // Add some data
        runStash("column", "add", "Name")
        runStash("add", "TestItem")

        // Try to init again
        _, _, err := runStash("init", "inventory", "--prefix", "inv-")
        require.Error(t, err)

        // Verify original data intact
        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, "TestItem")
    })
}
```

### Example: tests/usecases/records_test.go

```go
// TestUC_REC_001_AddRecord tests UC-REC-001
// See: usecases/records.yaml
func TestUC_REC_001_AddRecord(t *testing.T) {
    t.Run("AC-01: add record with primary value", func(t *testing.T) {
        // Given: Stash "inventory" exists with column "Name"
        setup := setupStash(t, "inventory", "inv-")
        runStash("column", "add", "Name")

        // When: User runs `stash add "Laptop"`
        stdout, _, err := runStash("add", "Laptop")
        require.NoError(t, err)

        // Then: Record is created with unique ID (inv-xxxx)
        id := strings.TrimSpace(stdout)
        assert.Regexp(t, `^inv-[a-z0-9]{4}$`, id)

        // Verify via show
        record := showRecord(t, id)

        // Then: Name field is set to "Laptop"
        assert.Equal(t, "Laptop", record.Fields["Name"])

        // Then: _hash is calculated
        assert.NotEmpty(t, record.Hash)
        assert.Len(t, record.Hash, 12)

        // Then: Audit fields are set
        assert.NotEmpty(t, record.CreatedAt)
        assert.NotEmpty(t, record.CreatedBy)
        assert.NotEmpty(t, record.UpdatedAt)
        assert.NotEmpty(t, record.UpdatedBy)
    })

    t.Run("AC-03: add child record", func(t *testing.T) {
        // Given: Record inv-ex4j exists in "inventory"
        setup := setupStash(t, "inventory", "inv-")
        runStash("column", "add", "Name")
        parentOut, _, _ := runStash("add", "Laptop")
        parentID := strings.TrimSpace(parentOut)

        // When: User runs `stash add "Charger" --parent inv-ex4j`
        childOut, _, err := runStash("add", "Charger", "--parent", parentID)
        require.NoError(t, err)

        // Then: Child record is created with ID inv-ex4j.1
        childID := strings.TrimSpace(childOut)
        assert.Equal(t, parentID+".1", childID)

        // Then: _parent field is set
        record := showRecord(t, childID)
        assert.Equal(t, parentID, record.ParentID)
    })
}

// TestUC_REC_001_MustNot tests anti-requirements
func TestUC_REC_001_AddRecord_MustNot(t *testing.T) {
    t.Run("must not create record without columns", func(t *testing.T) {
        setup := setupStash(t, "inventory", "inv-")
        // No columns added

        _, stderr, err := runStash("add", "Laptop")
        require.Error(t, err)
        assert.Contains(t, stderr, "no columns")
    })

    t.Run("must not create child without valid parent", func(t *testing.T) {
        setup := setupStash(t, "inventory", "inv-")
        runStash("column", "add", "Name")

        _, stderr, err := runStash("add", "Orphan", "--parent", "inv-fake")
        require.Error(t, err)
        assert.Contains(t, stderr, "not found")
    })
}
```

---

## Workflow

### Before Implementation

1. **Read the use case file:**
   ```bash
   cat usecases/records.yaml
   ```

2. **Output context block:**
   ```markdown
   ## Implementation Context
   **Use Case**: UC-REC-001 - Add Record
   **Acceptance Criteria**: AC-01, AC-02, AC-03, AC-04, AC-05
   **In Scope**: Creating record with ID, setting fields, child records
   **Out of Scope**: Bulk import, updating existing records
   **Must NOT**: Create without columns, allow duplicate IDs
   ```

3. **Write failing tests first** (one subtest per AC)

4. **Implement until tests pass**

5. **STOP** - Do not add anything not in acceptance criteria

### After Implementation: Drift Review

```markdown
## Drift Review for UC-REC-001

### Changes Made
1. Added `AddRecord` function → Maps to AC-01
2. Added `--set` flag handling → Maps to AC-02
3. Added `--parent` validation → Maps to AC-03, AC-04
4. Added JSON output → Maps to AC-05

### Verification
- [x] All code changes map to an acceptance criterion
- [x] No out_of_scope work was done
- [x] No must_not violations occurred
- [x] All AC subtests pass

### Unmapped Code (potential drift)
None
```

---

## Bug Attribution

When bugs are discovered, link them to use cases:

```bash
# Create bug in beads
bd create "Add record fails silently with empty value" --type bug

# Link to use case
bd label add <bug-id> uc:UC-REC-001

# Add AC gap analysis
bd comments add <bug-id> "AC Gap: UC-REC-001 should have AC for empty value handling"
```

### Label Convention

| Label | Meaning |
|-------|---------|
| `uc:UC-REC-001` | Bug relates to use case UC-REC-001 |
| `ac-gap` | Acceptance criteria was missing or incomplete |
| `must-not-violation` | Code violated a must_not constraint |

---

## Unit Tests (Non-UCDD)

Unit tests for internal functions don't need UCDD structure. Keep them simple:

```
tests/
├── unit/
│   ├── model/
│   │   ├── record_test.go      # ID generation, hash calculation
│   │   ├── column_test.go      # Name validation
│   │   └── stash_test.go       # Config serialization
│   ├── storage/
│   │   ├── jsonl_test.go       # Read/write/compact
│   │   ├── sqlite_test.go      # Table/column operations
│   │   └── sync_test.go        # Sync algorithm
│   └── output/
│       ├── json_test.go        # JSON formatting
│       ├── table_test.go       # Table formatting
│       └── tree_test.go        # Tree rendering
```

---

## Edge Cases and Failure Tests

Every use case must include tests for edge cases and failure scenarios. These are critical for robust software.

### EC-ST: Stash Management Edge Cases

```go
func TestUC_ST_EdgeCases(t *testing.T) {
    // EC-ST-001: Initialize with boundary prefix lengths
    t.Run("prefix exactly 2 chars", func(t *testing.T) {
        _, _, err := runStash("init", "test", "--prefix", "ab-")
        require.NoError(t, err) // Should succeed
    })

    t.Run("prefix exactly 4 chars", func(t *testing.T) {
        _, _, err := runStash("init", "test", "--prefix", "abcd-")
        require.NoError(t, err) // Should succeed
    })

    t.Run("prefix 1 char - too short", func(t *testing.T) {
        _, _, err := runStash("init", "test", "--prefix", "a-")
        require.Error(t, err)
    })

    t.Run("prefix 5 chars - too long", func(t *testing.T) {
        _, _, err := runStash("init", "test", "--prefix", "abcde-")
        require.Error(t, err)
    })

    // EC-ST-002: Special characters in stash name
    t.Run("stash name with hyphen", func(t *testing.T) {
        _, _, err := runStash("init", "my-stash", "--prefix", "ms-")
        require.NoError(t, err)
    })

    t.Run("stash name with underscore", func(t *testing.T) {
        _, _, err := runStash("init", "my_stash", "--prefix", "ms-")
        require.NoError(t, err)
    })

    t.Run("stash name with spaces - should fail", func(t *testing.T) {
        _, _, err := runStash("init", "my stash", "--prefix", "ms-")
        require.Error(t, err)
    })

    t.Run("stash name with special chars - should fail", func(t *testing.T) {
        _, _, err := runStash("init", "my@stash!", "--prefix", "ms-")
        require.Error(t, err)
    })

    // EC-ST-003: Empty/missing .stash directory
    t.Run("init creates .stash if not exists", func(t *testing.T) {
        tmpDir := t.TempDir()
        os.Chdir(tmpDir)
        assert.NoDirExists(t, ".stash")

        runStash("init", "test", "--prefix", "ts-")
        assert.DirExists(t, ".stash")
    })

    // EC-ST-004: Permissions issues
    t.Run("init fails with read-only directory", func(t *testing.T) {
        tmpDir := t.TempDir()
        os.Chmod(tmpDir, 0444)
        defer os.Chmod(tmpDir, 0755)

        os.Chdir(tmpDir)
        _, _, err := runStash("init", "test", "--prefix", "ts-")
        require.Error(t, err)
    })

    // EC-ST-005: Drop last stash behavior
    t.Run("drop last stash stops daemon", func(t *testing.T) {
        setupStash(t, "only-one", "oo-")

        runStash("drop", "only-one", "--yes")

        status, _, _ := runStash("daemon", "status")
        assert.Contains(t, status, "not running")
    })

    // EC-ST-006: Unicode in stash name
    t.Run("stash name with unicode - should fail", func(t *testing.T) {
        _, _, err := runStash("init", "测试", "--prefix", "ts-")
        require.Error(t, err)
    })
}
```

### EC-COL: Column Management Edge Cases

```go
func TestUC_COL_EdgeCases(t *testing.T) {
    // EC-COL-001: Column name boundaries
    t.Run("column name single char", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "X")
        require.NoError(t, err)
    })

    t.Run("column name 64 chars", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        longName := strings.Repeat("a", 64)
        _, _, err := runStash("column", "add", longName)
        require.NoError(t, err)
    })

    t.Run("column name 65 chars - too long", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        longName := strings.Repeat("a", 65)
        _, _, err := runStash("column", "add", longName)
        require.Error(t, err)
    })

    // EC-COL-002: All reserved column names
    reservedNames := []string{
        "_id", "_hash", "_parent", "_created_at", "_created_by",
        "_updated_at", "_updated_by", "_branch", "_deleted_at",
        "_deleted_by", "_op",
    }
    for _, name := range reservedNames {
        t.Run("reserved name: "+name, func(t *testing.T) {
            setup := setupStash(t, "test", "ts-")
            _, _, err := runStash("column", "add", name)
            require.Error(t, err)
            assert.Contains(t, err.Error(), "reserved")
        })
    }

    // EC-COL-003: Column names with various characters
    t.Run("column with underscore", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "my_column")
        require.NoError(t, err)
    })

    t.Run("column with numbers", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "column123")
        require.NoError(t, err)
    })

    t.Run("column starting with number - should fail", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "123column")
        require.Error(t, err)
    })

    t.Run("column with hyphen - should fail", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "my-column")
        require.Error(t, err)
    })

    // EC-COL-004: Case sensitivity
    t.Run("column names are case sensitive", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        _, _, err := runStash("column", "add", "name")
        // Depends on design: could allow both or reject as duplicate
    })

    // EC-COL-005: Description edge cases
    t.Run("empty description", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "Test", "--desc", "")
        require.NoError(t, err)
    })

    t.Run("very long description", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        longDesc := strings.Repeat("a", 1000)
        _, _, err := runStash("column", "add", "Test", "--desc", longDesc)
        require.NoError(t, err)
    })

    t.Run("description with special chars", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        _, _, err := runStash("column", "add", "Test", "--desc", "Test <>&\"' special")
        require.NoError(t, err)
    })

    // EC-COL-006: Rename to same name
    t.Run("rename column to itself", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        _, _, err := runStash("column", "rename", "Name", "Name")
        // Should either succeed (no-op) or fail gracefully
    })

    // EC-COL-007: Drop column with data
    t.Run("drop column preserves data in JSONL", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "ToDelete")
        runStash("add", "Test", "--set", "ToDelete", "value")

        runStash("column", "drop", "ToDelete", "--yes")

        // Verify JSONL still has the data
        content, _ := os.ReadFile(".stash/test/records.jsonl")
        assert.Contains(t, string(content), "ToDelete")
    })
}
```

### EC-REC: Record Operation Edge Cases

```go
func TestUC_REC_EdgeCases(t *testing.T) {
    // EC-REC-001: Empty and whitespace values
    t.Run("add record with empty value", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        _, _, err := runStash("add", "")
        // Design decision: allow or reject?
    })

    t.Run("add record with only whitespace", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        _, _, err := runStash("add", "   ")
        // Design decision: trim and reject, or allow?
    })

    // EC-REC-002: Very long values
    t.Run("add record with very long value", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        longValue := strings.Repeat("a", 10000)
        _, _, err := runStash("add", longValue)
        require.NoError(t, err)
    })

    // EC-REC-003: Special characters in values
    t.Run("value with quotes", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, err := runStash("add", `Value with "quotes"`)
        require.NoError(t, err)

        id := strings.TrimSpace(out)
        record := showRecord(t, id)
        assert.Equal(t, `Value with "quotes"`, record.Fields["Name"])
    })

    t.Run("value with newlines", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, err := runStash("add", "Line1\nLine2\nLine3")
        require.NoError(t, err)

        id := strings.TrimSpace(out)
        record := showRecord(t, id)
        assert.Contains(t, record.Fields["Name"], "\n")
    })

    t.Run("value with unicode", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, err := runStash("add", "日本語テスト 🎉")
        require.NoError(t, err)

        id := strings.TrimSpace(out)
        record := showRecord(t, id)
        assert.Equal(t, "日本語テスト 🎉", record.Fields["Name"])
    })

    t.Run("value with null bytes - should fail or sanitize", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        _, _, err := runStash("add", "test\x00value")
        // Should either reject or sanitize null bytes
    })

    // EC-REC-004: Numeric and boolean values
    t.Run("set numeric value", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Count")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        runStash("set", id, "Count", "42")
        record := showRecord(t, id)
        // Verify it's stored correctly (as number or string)
    })

    t.Run("set boolean value", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Active")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        runStash("set", id, "Active", "true")
        record := showRecord(t, id)
        // Verify it's stored correctly
    })

    // EC-REC-005: ID collision handling
    t.Run("ID generation handles collisions", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        ids := make(map[string]bool)
        for i := 0; i < 1000; i++ {
            out, _, err := runStash("add", fmt.Sprintf("Record%d", i))
            require.NoError(t, err)
            id := strings.TrimSpace(out)
            assert.False(t, ids[id], "Duplicate ID generated: %s", id)
            ids[id] = true
        }
    })

    // EC-REC-006: Deep hierarchy
    t.Run("create 10-level deep hierarchy", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        parentID := ""
        for i := 0; i < 10; i++ {
            args := []string{"add", fmt.Sprintf("Level%d", i)}
            if parentID != "" {
                args = append(args, "--parent", parentID)
            }
            out, _, err := runStash(args...)
            require.NoError(t, err)
            parentID = strings.TrimSpace(out)
        }

        // Verify tree structure
        out, _, _ := runStash("list", "--tree")
        assert.Contains(t, out, "Level9")
    })

    // EC-REC-007: Delete/restore cycles
    t.Run("delete and restore multiple times", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        for i := 0; i < 5; i++ {
            runStash("delete", id, "--yes")
            runStash("restore", id)
        }

        // Verify record is active
        out, _, _ = runStash("list", "--json")
        assert.Contains(t, out, id)
    })

    // EC-REC-008: Update deleted record
    t.Run("cannot update deleted record", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Value")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        runStash("delete", id, "--yes")

        _, stderr, err := runStash("set", id, "Value", "new")
        require.Error(t, err)
        assert.Contains(t, stderr, "deleted")
    })

    // EC-REC-009: Purge with children
    t.Run("purge parent also purges children", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        parentOut, _, _ := runStash("add", "Parent")
        parentID := strings.TrimSpace(parentOut)
        childOut, _, _ := runStash("add", "Child", "--parent", parentID)
        childID := strings.TrimSpace(childOut)

        runStash("delete", parentID, "--cascade", "--yes")
        runStash("purge", "--id", parentID, "--yes")

        // Both should be gone
        _, _, err := runStash("show", parentID)
        require.Error(t, err)
        _, _, err = runStash("show", childID)
        require.Error(t, err)
    })

    // EC-REC-010: Hash consistency
    t.Run("same content produces same hash", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Value")

        out1, _, _ := runStash("add", "Test", "--set", "Value", "123", "--json")
        out2, _, _ := runStash("add", "Test", "--set", "Value", "123", "--json")

        var rec1, rec2 map[string]interface{}
        json.Unmarshal([]byte(out1), &rec1)
        json.Unmarshal([]byte(out2), &rec2)

        assert.Equal(t, rec1["_hash"], rec2["_hash"])
    })

    t.Run("different content produces different hash", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Value")

        out1, _, _ := runStash("add", "Test", "--set", "Value", "123", "--json")
        out2, _, _ := runStash("add", "Test", "--set", "Value", "456", "--json")

        var rec1, rec2 map[string]interface{}
        json.Unmarshal([]byte(out1), &rec1)
        json.Unmarshal([]byte(out2), &rec2)

        assert.NotEqual(t, rec1["_hash"], rec2["_hash"])
    })
}
```

### EC-QRY: Query Edge Cases

```go
func TestUC_QRY_EdgeCases(t *testing.T) {
    // EC-QRY-001: Empty results
    t.Run("list on empty stash", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        out, _, err := runStash("list", "--json")
        require.NoError(t, err)
        assert.Equal(t, "[]\n", out)
    })

    t.Run("where clause returns no results", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Test")

        out, _, err := runStash("list", "--where", "Name = 'NonExistent'", "--json")
        require.NoError(t, err)
        assert.Equal(t, "[]\n", out)
    })

    // EC-QRY-002: SQL injection attempts
    sqlInjections := []string{
        "'; DROP TABLE test; --",
        "1 OR 1=1",
        "1; DELETE FROM test",
        "1 UNION SELECT * FROM sqlite_master",
    }
    for _, injection := range sqlInjections {
        t.Run("SQL injection: "+injection[:20], func(t *testing.T) {
            setup := setupStash(t, "test", "ts-")
            runStash("column", "add", "Name")
            runStash("add", "Test")

            // Should not execute malicious SQL
            _, _, err := runStash("list", "--where", fmt.Sprintf("Name = '%s'", injection))
            // Should either fail safely or return no results
            // Should NOT drop tables or modify data

            // Verify table still exists
            out, _, _ := runStash("list", "--json")
            assert.Contains(t, out, "Test")
        })
    }

    // EC-QRY-003: Complex WHERE clauses
    t.Run("where with AND", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Type", "Active")
        runStash("add", "A", "--set", "Type", "x", "--set", "Active", "true")
        runStash("add", "B", "--set", "Type", "x", "--set", "Active", "false")
        runStash("add", "C", "--set", "Type", "y", "--set", "Active", "true")

        out, _, _ := runStash("list", "--where", "Type = 'x' AND Active = 'true'", "--json")
        assert.Contains(t, out, "A")
        assert.NotContains(t, out, "B")
        assert.NotContains(t, out, "C")
    })

    t.Run("where with OR", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Type")
        runStash("add", "A", "--set", "Type", "x")
        runStash("add", "B", "--set", "Type", "y")
        runStash("add", "C", "--set", "Type", "z")

        out, _, _ := runStash("list", "--where", "Type = 'x' OR Type = 'y'", "--json")
        assert.Contains(t, out, "A")
        assert.Contains(t, out, "B")
        assert.NotContains(t, out, "C")
    })

    t.Run("where with LIKE", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "MacBook Pro")
        runStash("add", "MacBook Air")
        runStash("add", "iPhone")

        out, _, _ := runStash("list", "--where", "Name LIKE 'Mac%'", "--json")
        assert.Contains(t, out, "MacBook Pro")
        assert.Contains(t, out, "MacBook Air")
        assert.NotContains(t, out, "iPhone")
    })

    t.Run("where with NULL check", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Optional")
        runStash("add", "A", "--set", "Optional", "set")
        runStash("add", "B") // Optional not set

        out, _, _ := runStash("list", "--where", "Optional IS NULL", "--json")
        assert.NotContains(t, out, "A")
        assert.Contains(t, out, "B")
    })

    // EC-QRY-004: Invalid WHERE syntax
    t.Run("invalid where syntax", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        _, stderr, err := runStash("list", "--where", "Name = ")
        require.Error(t, err)
    })

    // EC-QRY-005: Query with non-existent column
    t.Run("where references non-existent column", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        _, stderr, err := runStash("list", "--where", "FakeColumn = 'test'")
        require.Error(t, err)
        assert.Contains(t, stderr, "FakeColumn")
    })

    // EC-QRY-006: Raw query safety
    t.Run("query blocks DELETE", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Test")

        _, _, err := runStash("query", "DELETE FROM test")
        require.Error(t, err)

        // Verify data intact
        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, "Test")
    })

    t.Run("query blocks UPDATE", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Test")

        _, _, err := runStash("query", "UPDATE test SET Name = 'Hacked'")
        require.Error(t, err)
    })

    t.Run("query blocks INSERT", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        _, _, err := runStash("query", "INSERT INTO test (Name) VALUES ('Hacked')")
        require.Error(t, err)
    })

    t.Run("query blocks DROP", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")

        _, _, err := runStash("query", "DROP TABLE test")
        require.Error(t, err)
    })

    // EC-QRY-007: History edge cases
    t.Run("history of never-modified record", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        history, _, _ := runStash("history", id)
        // Should show only create operation
        assert.Contains(t, history, "create")
        assert.NotContains(t, history, "update")
    })

    t.Run("history of purged record", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        runStash("delete", id, "--yes")
        runStash("purge", "--id", id, "--yes")

        _, _, err := runStash("history", id)
        require.Error(t, err)
    })
}
```

### EC-IMP: Import/Export Edge Cases

```go
func TestUC_IMP_EdgeCases(t *testing.T) {
    // EC-IMP-001: CSV format edge cases
    t.Run("CSV with quoted commas", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Description
"Product A","A, B, and C"
"Product B","Simple desc"
`)
        _, _, err := runStash("import", "test.csv", "--confirm")
        require.NoError(t, err)

        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, "A, B, and C")
    })

    t.Run("CSV with embedded quotes", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Description
"Product A","He said ""Hello"""
`)
        _, _, err := runStash("import", "test.csv", "--confirm")
        require.NoError(t, err)

        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, `He said "Hello"`)
    })

    t.Run("CSV with newlines in values", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Description
"Product A","Line 1
Line 2
Line 3"
`)
        _, _, err := runStash("import", "test.csv", "--confirm")
        require.NoError(t, err)
    })

    t.Run("CSV with BOM", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        content := "\xef\xbb\xbfName,Price\nTest,100\n"
        os.WriteFile("test.csv", []byte(content), 0644)

        _, _, err := runStash("import", "test.csv", "--confirm")
        require.NoError(t, err)

        // Verify column name doesn't include BOM
        out, _, _ := runStash("column", "list", "--json")
        assert.Contains(t, out, `"name":"Name"`)
        assert.NotContains(t, out, "\xef\xbb\xbf")
    })

    t.Run("CSV with empty rows", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Price
Product A,100

Product B,200

`)
        _, _, err := runStash("import", "test.csv", "--confirm")
        require.NoError(t, err)

        out, _, _ := runStash("list", "--json")
        records := parseJSON(out)
        assert.Len(t, records, 2) // Empty rows ignored
    })

    t.Run("CSV with inconsistent columns", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Price
Product A,100
Product B
Product C,200,Extra
`)
        // Design decision: fail, or handle gracefully?
    })

    // EC-IMP-002: Empty CSV
    t.Run("import empty CSV", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Price
`)
        _, _, err := runStash("import", "test.csv", "--confirm")
        // Should succeed with 0 records, or fail gracefully
    })

    t.Run("import headers-only CSV", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        writeCSV(t, "test.csv", `Name,Price`)

        _, _, err := runStash("import", "test.csv", "--confirm")
        // Should add columns but import 0 records
    })

    // EC-IMP-003: Large CSV
    t.Run("import 10000 row CSV", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")

        var buf bytes.Buffer
        buf.WriteString("Name,Value\n")
        for i := 0; i < 10000; i++ {
            buf.WriteString(fmt.Sprintf("Item%d,%d\n", i, i))
        }
        os.WriteFile("large.csv", buf.Bytes(), 0644)

        start := time.Now()
        _, _, err := runStash("import", "large.csv", "--confirm")
        duration := time.Since(start)

        require.NoError(t, err)
        assert.Less(t, duration, 30*time.Second) // Performance check

        out, _, _ := runStash("query", "SELECT COUNT(*) as c FROM test", "--json")
        assert.Contains(t, out, "10000")
    })

    // EC-IMP-004: Export edge cases
    t.Run("export empty stash", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        _, _, err := runStash("export", "out.csv")
        require.NoError(t, err)

        content, _ := os.ReadFile("out.csv")
        lines := strings.Split(string(content), "\n")
        assert.Equal(t, "Name", strings.TrimSpace(lines[0])) // Header only
    })

    t.Run("export with special chars in values", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", `Test "with" quotes, and commas`)

        _, _, err := runStash("export", "out.csv")
        require.NoError(t, err)

        // Re-import should preserve values
        runStash("drop", "test", "--yes")
        runStash("init", "test2", "--prefix", "t2-")
        runStash("import", "out.csv", "--confirm")

        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, `Test "with" quotes, and commas`)
    })
}
```

### EC-SYN: Sync and Maintenance Edge Cases

```go
func TestUC_SYN_EdgeCases(t *testing.T) {
    // EC-SYN-001: Corrupted JSONL
    t.Run("sync with malformed JSONL line", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Valid")

        // Corrupt JSONL
        f, _ := os.OpenFile(".stash/test/records.jsonl", os.O_APPEND|os.O_WRONLY, 0644)
        f.WriteString("not valid json\n")
        f.Close()

        _, stderr, err := runStash("sync")
        require.Error(t, err)
        assert.Contains(t, stderr, "malformed")
    })

    t.Run("doctor detects malformed JSONL", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Valid")

        // Corrupt JSONL
        f, _ := os.OpenFile(".stash/test/records.jsonl", os.O_APPEND|os.O_WRONLY, 0644)
        f.WriteString("not valid json\n")
        f.Close()

        out, _, _ := runStash("doctor")
        assert.Contains(t, out, "malformed")
    })

    // EC-SYN-002: Missing files
    t.Run("doctor detects missing attached files", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Doc")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)
        runStash("file", id, "Doc", "--content", "test")

        // Delete the file
        os.Remove(".stash/test/files/" + id + ".md")

        doctorOut, _, _ := runStash("doctor")
        assert.Contains(t, doctorOut, "missing")
    })

    t.Run("doctor --fix removes dangling references", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Doc")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)
        runStash("file", id, "Doc", "--content", "test")

        // Delete the file
        os.Remove(".stash/test/files/" + id + ".md")

        runStash("doctor", "--fix", "--yes")

        // Reference should be removed
        record := showRecord(t, id)
        assert.Empty(t, record.Fields["Doc"])
    })

    // EC-SYN-003: Orphaned files
    t.Run("doctor detects orphaned files", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        // Create orphaned file
        os.WriteFile(".stash/test/files/orphan.md", []byte("orphan"), 0644)

        out, _, _ := runStash("doctor")
        assert.Contains(t, out, "orphan")
    })

    t.Run("repair --clean-orphans removes orphaned files", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")

        // Create orphaned file
        os.MkdirAll(".stash/test/files", 0755)
        os.WriteFile(".stash/test/files/orphan.md", []byte("orphan"), 0644)

        runStash("repair", "--clean-orphans", "--yes")

        _, err := os.Stat(".stash/test/files/orphan.md")
        assert.True(t, os.IsNotExist(err))
    })

    // EC-SYN-004: Hash mismatch
    t.Run("doctor detects hash mismatch", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        out, _, _ := runStash("add", "Test")
        id := strings.TrimSpace(out)

        // Manually modify SQLite without updating hash
        // (simulate external modification)

        doctorOut, _, _ := runStash("doctor", "--deep")
        // Should detect mismatch
    })

    t.Run("repair --rehash fixes hash mismatches", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")
        runStash("add", "Test")

        runStash("repair", "--rehash")
        runStash("doctor", "--deep")
        // Should be clean now
    })

    // EC-SYN-005: Concurrent access
    t.Run("concurrent writes don't corrupt data", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name", "Value")

        var wg sync.WaitGroup
        errors := make(chan error, 100)

        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(n int) {
                defer wg.Done()
                for j := 0; j < 10; j++ {
                    _, _, err := runStash("add", fmt.Sprintf("Record-%d-%d", n, j))
                    if err != nil {
                        errors <- err
                    }
                }
            }(i)
        }

        wg.Wait()
        close(errors)

        for err := range errors {
            t.Errorf("Concurrent write error: %v", err)
        }

        // Verify all records exist
        out, _, _ := runStash("query", "SELECT COUNT(*) as c FROM test", "--json")
        assert.Contains(t, out, "100")
    })

    // EC-SYN-006: Daemon not running
    t.Run("commands work without daemon", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-", "--no-daemon")

        _, _, err := runStash("column", "add", "Name")
        require.NoError(t, err)

        _, _, err = runStash("add", "Test")
        require.NoError(t, err)

        out, _, _ := runStash("list", "--json")
        assert.Contains(t, out, "Test")
    })
}
```

### EC-DMN: Daemon Edge Cases

```go
func TestUC_DMN_EdgeCases(t *testing.T) {
    // EC-DMN-001: Start when already running
    t.Run("start when already running", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")

        runStash("daemon", "start")
        _, stderr, err := runStash("daemon", "start")
        // Should either succeed (no-op) or fail gracefully
    })

    // EC-DMN-002: Stop when not running
    t.Run("stop when not running", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-", "--no-daemon")

        _, _, err := runStash("daemon", "stop")
        // Should succeed (no-op) or fail gracefully
    })

    // EC-DMN-003: Stale PID file
    t.Run("handles stale PID file", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-", "--no-daemon")

        // Create stale PID file with non-existent PID
        os.WriteFile(".stash/daemon.pid", []byte("99999999"), 0644)

        _, _, err := runStash("daemon", "start")
        require.NoError(t, err) // Should detect stale PID and start anyway
    })

    // EC-DMN-004: Log rotation
    t.Run("log file doesn't grow unbounded", func(t *testing.T) {
        setup := setupStash(t, "test", "ts-")
        runStash("column", "add", "Name")

        // Generate lots of activity
        for i := 0; i < 1000; i++ {
            runStash("add", fmt.Sprintf("Record%d", i))
        }

        info, _ := os.Stat(".stash/daemon.log")
        assert.Less(t, info.Size(), int64(10*1024*1024)) // < 10MB
    })
}
```

---

## Failure Test Summary

| Category | Test Count | Coverage |
|----------|------------|----------|
| Stash Management | 15 | Invalid prefixes, permissions, unicode |
| Column Management | 20 | Reserved names, boundaries, case sensitivity |
| Record Operations | 25 | Empty values, unicode, deep hierarchy, hash |
| Querying | 18 | SQL injection, NULL handling, blocked operations |
| Import/Export | 15 | CSV edge cases, BOM, large files |
| Sync/Maintenance | 12 | Corruption, orphans, concurrent access |
| Daemon | 8 | Stale PID, already running |

**Total: ~113 edge case and failure tests**

---

## Coverage Requirements

| Category | Requirement |
|----------|-------------|
| UCDD tests | 100% of acceptance criteria |
| Unit tests | > 80% line coverage |
| Must-not tests | All must_not items have tests |
| Edge case tests | All categories above |
| Failure tests | All error codes tested |

---

## CI Integration

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      # Run all tests
      - run: go test -v -race -coverprofile=coverage.out ./...

      # Check coverage
      - run: go tool cover -func=coverage.out

      # Verify UCDD coverage (custom script)
      - run: ./scripts/verify-ucdd-coverage.sh
```
