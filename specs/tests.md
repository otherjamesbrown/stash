# Stash: Test Suite Design

This document defines the test strategy and test cases for Stash.

---

## Test Strategy

### Test Levels

1. **Unit Tests** - Individual functions and methods
2. **Integration Tests** - Component interactions (JSONL ↔ SQLite)
3. **CLI Tests** - End-to-end command testing
4. **Scenario Tests** - Full use case workflows

### Test Organization

```
tests/
├── unit/
│   ├── model/
│   │   ├── record_test.go      # Record type, ID generation
│   │   ├── column_test.go      # Column type, validation
│   │   └── stash_test.go       # Stash config operations
│   ├── storage/
│   │   ├── jsonl_test.go       # JSONL read/write/compact
│   │   ├── sqlite_test.go      # SQLite operations
│   │   └── sync_test.go        # Sync logic
│   └── output/
│       ├── json_test.go        # JSON formatting
│       ├── table_test.go       # Table formatting
│       └── tree_test.go        # Tree formatting
├── integration/
│   ├── storage_sync_test.go    # JSONL ↔ SQLite sync
│   ├── daemon_test.go          # Daemon file watching
│   └── concurrent_test.go      # Concurrent access
├── cli/
│   ├── init_test.go            # stash init
│   ├── add_test.go             # stash add
│   ├── set_test.go             # stash set
│   ├── list_test.go            # stash list
│   ├── column_test.go          # stash column *
│   ├── import_test.go          # stash import
│   ├── export_test.go          # stash export
│   └── maintenance_test.go     # sync, doctor, repair
└── scenarios/
    ├── company_research_test.go    # Use Case 1
    ├── hierarchy_test.go           # Use Case 2
    ├── multi_stash_test.go         # Use Case 4
    └── recovery_test.go            # Use Case 10
```

---

## Unit Tests

### model/record_test.go

```go
func TestGenerateID(t *testing.T)
    // Test: generates valid base36 ID
    // Test: ID has correct prefix
    // Test: no collisions in 10,000 generations
    // Test: child ID format correct (parent.seq)

func TestParseID(t *testing.T)
    // Test: parse root ID "re-ex4j"
    // Test: parse child ID "re-ex4j.1"
    // Test: parse deep child "re-ex4j.1.2.3"
    // Test: invalid ID returns error

func TestRecordValidation(t *testing.T)
    // Test: valid record passes
    // Test: missing _id fails
    // Test: invalid _op fails
    // Test: child with missing parent detected
```

### model/column_test.go

```go
func TestColumnNameValidation(t *testing.T)
    // Test: valid names pass (Name, company_name, Column1)
    // Test: reserved names fail (_id, _ts, _op, _parent, _deleted)
    // Test: invalid chars fail (spaces, special chars)
    // Test: empty name fails

func TestColumnDescription(t *testing.T)
    // Test: description stored correctly
    // Test: description can be updated
    // Test: empty description allowed
```

### model/stash_test.go

```go
func TestStashConfig(t *testing.T)
    // Test: create config with required fields
    // Test: prefix validation (2-4 chars, alphanumeric)
    // Test: columns list operations (add, remove, rename)

func TestConfigPersistence(t *testing.T)
    // Test: save to JSON
    // Test: load from JSON
    // Test: round-trip preserves data
```

### storage/jsonl_test.go

```go
func TestJSONLAppend(t *testing.T)
    // Test: append single record
    // Test: append multiple records
    // Test: append preserves order
    // Test: file created if not exists

func TestJSONLRead(t *testing.T)
    // Test: read empty file returns empty slice
    // Test: read single record
    // Test: read multiple records
    // Test: malformed line returns error with line number

func TestJSONLCompact(t *testing.T)
    // Test: multiple updates become single record
    // Test: deleted records removed
    // Test: order preserved (by _ts)
    // Test: original file backed up
```

### storage/sqlite_test.go

```go
func TestEnsureTable(t *testing.T)
    // Test: creates table with base columns
    // Test: idempotent (no error if exists)
    // Test: creates indexes

func TestEnsureColumn(t *testing.T)
    // Test: adds column via ALTER TABLE
    // Test: idempotent (no error if exists)
    // Test: sanitizes column name

func TestUpsert(t *testing.T)
    // Test: insert new record
    // Test: update existing record
    // Test: handles NULL values
    // Test: handles all JSON types (string, number, bool)

func TestQuery(t *testing.T)
    // Test: SELECT * works
    // Test: WHERE clause filters
    // Test: ORDER BY works
    // Test: JOIN (for hierarchy) works
    // Test: invalid SQL returns error
```

### storage/sync_test.go

```go
func TestSyncJSONLToSQLite(t *testing.T)
    // Test: empty JSONL creates empty table
    // Test: new records inserted
    // Test: updates applied in order
    // Test: deletes mark records

func TestSyncSQLiteToJSONL(t *testing.T)
    // Test: DB changes appended to JSONL
    // Test: preserves existing JSONL entries

func TestSyncConflictDetection(t *testing.T)
    // Test: same record modified in both detected
    // Test: different records no conflict
    // Test: conflict resolution (prefer JSONL)
    // Test: conflict resolution (prefer DB)
```

---

## Integration Tests

### storage_sync_test.go

```go
func TestFullSyncCycle(t *testing.T)
    // 1. Create records via JSONL append
    // 2. Sync to SQLite
    // 3. Query SQLite
    // 4. Modify via SQLite
    // 5. Sync back to JSONL
    // 6. Verify JSONL has changes

func TestConcurrentWrites(t *testing.T)
    // 1. Start 10 goroutines
    // 2. Each appends 100 records to JSONL
    // 3. Sync to SQLite
    // 4. Verify all 1000 records present
    // 5. No duplicates, no data loss
```

### daemon_test.go

```go
func TestDaemonStartStop(t *testing.T)
    // Test: start creates PID file
    // Test: stop removes PID file
    // Test: restart works

func TestDaemonFileWatch(t *testing.T)
    // 1. Start daemon
    // 2. Append to JSONL externally
    // 3. Wait for sync (with timeout)
    // 4. Verify SQLite updated

func TestDaemonGracefulShutdown(t *testing.T)
    // Test: pending syncs complete before exit
    // Test: no data loss on SIGTERM
```

---

## CLI Tests

### init_test.go

```go
func TestStashInit(t *testing.T)
    // Test: creates .stash directory
    // Test: creates stash subdirectory
    // Test: creates config.json
    // Test: creates empty records.jsonl
    // Test: starts daemon
    // Test: --no-daemon skips daemon

func TestStashInitValidation(t *testing.T)
    // Test: requires --prefix
    // Test: rejects invalid prefix (too short, too long, special chars)
    // Test: rejects duplicate stash name

func TestStashDrop(t *testing.T)
    // Test: removes stash directory
    // Test: removes from SQLite
    // Test: --yes skips confirmation
```

### add_test.go

```go
func TestStashAdd(t *testing.T)
    // Test: creates record with ID
    // Test: stores primary value in first column
    // Test: --set adds additional columns
    // Test: --parent creates child record
    // Test: outputs ID (plain)
    // Test: outputs JSON (--json)

func TestStashAddHierarchy(t *testing.T)
    // Test: child gets parent.1 ID
    // Test: second child gets parent.2
    // Test: grandchild gets parent.1.1
    // Test: --parent with invalid ID fails
```

### set_test.go

```go
func TestStashSet(t *testing.T)
    // Test: updates existing field
    // Test: adds new field (column exists)
    // Test: fails if column doesn't exist
    // Test: fails if record doesn't exist
    // Test: updates timestamp

func TestStashSetMultiple(t *testing.T)
    // Test: --col flag for multiple columns
```

### list_test.go

```go
func TestStashList(t *testing.T)
    // Test: lists all records
    // Test: --where filters
    // Test: --columns selects columns
    // Test: --json outputs JSON array
    // Test: empty stash returns empty

func TestStashListTree(t *testing.T)
    // Test: --tree shows hierarchy
    // Test: indentation correct
    // Test: children under parents

func TestStashListWhere(t *testing.T)
    // Test: equality "Verified = true"
    // Test: IS NULL
    // Test: IS NOT NULL
    // Test: LIKE pattern
    // Test: comparison operators
```

### column_test.go

```go
func TestColumnAdd(t *testing.T)
    // Test: adds single column
    // Test: adds multiple columns
    // Test: --desc sets description
    // Test: rejects duplicate column
    // Test: rejects reserved names

func TestColumnList(t *testing.T)
    // Test: shows all columns
    // Test: shows descriptions
    // Test: shows populated/empty counts
    // Test: --json outputs JSON array

func TestColumnRename(t *testing.T)
    // Test: renames in config
    // Test: renames in SQLite (via ALTER TABLE)
    // Test: data preserved

func TestColumnDescribe(t *testing.T)
    // Test: sets new description
    // Test: updates existing description

func TestColumnDrop(t *testing.T)
    // Test: removes from config
    // Test: removes from SQLite
    // Test: --yes skips confirmation
    // Test: data preserved in JSONL
```

### import_test.go

```go
func TestStashImport(t *testing.T)
    // Test: imports CSV
    // Test: creates columns from headers
    // Test: creates records with IDs
    // Test: --column specifies primary
    // Test: --confirm skips prompt
    // Test: --dry-run shows preview

func TestImportValidation(t *testing.T)
    // Test: detects existing columns
    // Test: shows new columns
    // Test: shows row counts
    // Test: handles empty values

func TestImportEdgeCases(t *testing.T)
    // Test: CSV with quotes
    // Test: CSV with commas in values
    // Test: CSV with empty rows
    // Test: CSV with BOM
```

### export_test.go

```go
func TestStashExport(t *testing.T)
    // Test: exports to CSV
    // Test: exports to JSON
    // Test: --where filters
    // Test: includes all columns

func TestExportFormats(t *testing.T)
    // Test: CSV properly escaped
    // Test: JSON properly formatted
```

### maintenance_test.go

```go
func TestStashSync(t *testing.T)
    // Test: --status shows state
    // Test: --rebuild recreates SQLite
    // Test: --flush exports to JSONL

func TestStashDoctor(t *testing.T)
    // Test: detects missing files
    // Test: detects orphaned files
    // Test: detects schema mismatch
    // Test: detects sync issues
    // Test: --fix repairs issues
    // Test: --json outputs JSON

func TestStashRepair(t *testing.T)
    // Test: --source jsonl rebuilds from JSONL
    // Test: --source db rebuilds JSONL from DB
    // Test: --clean-orphans removes files
    // Test: --dry-run previews changes
```

---

## Scenario Tests

Based on use cases document.

### company_research_test.go (Use Case 1)

```go
func TestCompanyResearchPipeline(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()

    // Create test CSV
    csvContent := `CompanyName,Industry
Microsoft,Technology
Apple,Technology
Acme Corp,Unknown`
    writeFile(tmpDir, "companies.csv", csvContent)

    // Init stash
    runCmd("stash", "init", "research", "--prefix", "re-")

    // Import
    runCmd("stash", "import", "companies.csv", "--confirm")

    // Verify import
    output := runCmd("stash", "list", "--json")
    records := parseJSON(output)
    assert.Len(t, records, 3)

    // Add research columns
    runCmd("stash", "column", "add", "Verified", "Overview")

    // Simulate research
    for _, r := range records {
        id := r["id"].(string)
        runCmd("stash", "set", id, "Verified", "true")
    }

    // Query verified
    output = runCmd("stash", "list", "--where", "Verified = 'true'", "--json")
    verified := parseJSON(output)
    assert.Len(t, verified, 3)
}
```

### hierarchy_test.go (Use Case 2)

```go
func TestHierarchicalCatalog(t *testing.T) {
    runCmd("stash", "init", "catalog", "--prefix", "cat-")
    runCmd("stash", "column", "add", "Name", "Type", "Price")

    // Create hierarchy
    out := runCmd("stash", "add", "Electronics", "--set", "Type", "category")
    parentID := strings.TrimSpace(out)  // cat-xxxx

    out = runCmd("stash", "add", "Computers", "--parent", parentID, "--set", "Type", "subcategory")
    childID := strings.TrimSpace(out)  // cat-xxxx.1

    out = runCmd("stash", "add", "MacBook", "--parent", childID, "--set", "Type", "product")
    grandchildID := strings.TrimSpace(out)  // cat-xxxx.1.1

    // Verify IDs
    assert.True(t, strings.HasPrefix(childID, parentID+"."))
    assert.True(t, strings.HasPrefix(grandchildID, childID+"."))

    // Verify tree
    out = runCmd("stash", "list", "--tree")
    assert.Contains(t, out, "Electronics")
    assert.Contains(t, out, "├─")
    assert.Contains(t, out, "Computers")

    // Verify children query
    out = runCmd("stash", "children", parentID, "--json")
    children := parseJSON(out)
    assert.Len(t, children, 1)
    assert.Equal(t, "Computers", children[0]["Name"])
}
```

### multi_stash_test.go (Use Case 4)

```go
func TestMultiStashWorkflow(t *testing.T) {
    // Create two stashes
    runCmd("stash", "init", "companies", "--prefix", "co-")
    runCmd("stash", "init", "contacts", "--prefix", "ct-")

    // Add to companies
    runCmd("stash", "column", "add", "--stash", "companies", "Name", "Website")
    out := runCmd("stash", "add", "--stash", "companies", "Acme Inc")
    companyID := strings.TrimSpace(out)

    // Add to contacts with reference
    runCmd("stash", "column", "add", "--stash", "contacts", "Name", "Email", "Company")
    runCmd("stash", "add", "--stash", "contacts", "John Smith",
        "--set", "Company", companyID)

    // Query contacts by company
    out = runCmd("stash", "list", "--stash", "contacts",
        "--where", fmt.Sprintf("Company = '%s'", companyID), "--json")
    contacts := parseJSON(out)
    assert.Len(t, contacts, 1)
    assert.Equal(t, "John Smith", contacts[0]["Name"])

    // Verify info shows both
    out = runCmd("stash", "info")
    assert.Contains(t, out, "companies (co-)")
    assert.Contains(t, out, "contacts (ct-)")
}
```

### recovery_test.go (Use Case 10)

```go
func TestRecoveryFromCorruption(t *testing.T) {
    // Setup stash with data
    runCmd("stash", "init", "test", "--prefix", "t-")
    runCmd("stash", "column", "add", "Name")
    runCmd("stash", "add", "Record1")
    runCmd("stash", "add", "Record2")

    // Corrupt SQLite by deleting it
    os.Remove(".stash/cache.db")

    // Doctor should detect
    out := runCmd("stash", "doctor")
    assert.Contains(t, out, "cache database")

    // Repair from JSONL
    runCmd("stash", "repair", "--source", "jsonl")

    // Verify data restored
    out = runCmd("stash", "list", "--json")
    records := parseJSON(out)
    assert.Len(t, records, 2)
}

func TestRecoveryFromJSONLCorruption(t *testing.T) {
    // Setup stash with data
    runCmd("stash", "init", "test", "--prefix", "t-")
    runCmd("stash", "column", "add", "Name")
    runCmd("stash", "add", "Record1")

    // Corrupt JSONL by adding invalid line
    f, _ := os.OpenFile(".stash/test/records.jsonl", os.O_APPEND|os.O_WRONLY, 0644)
    f.WriteString("invalid json\n")
    f.Close()

    // Doctor should detect
    out := runCmd("stash", "doctor")
    assert.Contains(t, out, "malformed")

    // Repair from DB
    runCmd("stash", "repair", "--source", "db")

    // Verify JSONL fixed
    out = runCmd("stash", "list", "--json")
    records := parseJSON(out)
    assert.Len(t, records, 1)
}
```

---

## Test Fixtures

### fixtures/companies.csv

```csv
CompanyName,Industry,Website
Microsoft,Technology,https://microsoft.com
Apple,Technology,https://apple.com
Acme Corp,Manufacturing,
"Company, Inc",Services,https://company.com
```

### fixtures/large-dataset.csv

Generated: 10,000 rows for performance testing.

### fixtures/corrupted.jsonl

```jsonl
{"_id":"t-a1b2","_ts":"2025-01-08T10:00:00Z","_op":"create","Name":"Valid"}
invalid json line
{"_id":"t-c3d4","_ts":"2025-01-08T10:01:00Z","_op":"create","Name":"Also Valid"}
```

---

## Test Utilities

### testutil/helpers.go

```go
// SetupTestStash creates a temp directory with initialized stash
func SetupTestStash(t *testing.T, name, prefix string) string

// RunStash executes stash command and returns output
func RunStash(args ...string) (stdout, stderr string, err error)

// ParseJSONOutput parses JSON array output
func ParseJSONOutput(output string) []map[string]interface{}

// AssertRecordExists checks record exists with expected fields
func AssertRecordExists(t *testing.T, id string, expected map[string]interface{})

// AssertFileContains checks file attachment content
func AssertFileContains(t *testing.T, stash, id, column, content string)

// WaitForSync waits for daemon sync with timeout
func WaitForSync(t *testing.T, timeout time.Duration)
```

---

## Coverage Requirements

- **Unit tests**: > 80% line coverage
- **Integration tests**: All sync scenarios covered
- **CLI tests**: All commands and flags tested
- **Scenario tests**: All use cases from use-cases.md

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
      - run: go test -v -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out
```
