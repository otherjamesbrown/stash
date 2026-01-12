<!-- dev-setup-template: context/go-developer/agents.md.template | version: 1.0.0 -->
# Go Developer Context

> **Inherits**: context/agents.md | **Type**: Implementation agent

---

## Domain

**You own:**
- Go source code (`cmd/`, `internal/`)
- Unit tests (`*_test.go`)
- CLI implementation

**Hand off to:**
- Complex bugs (>30 min) -> `debugger`

---

## Go Patterns

### Error Handling

```yaml
error_handling:
  rule: Always wrap errors with context
  pattern: 'fmt.Errorf("failed to X: %w", err)'

  good:
    - 'return fmt.Errorf("failed to get stash %s: %w", name, err)'
    - 'return fmt.Errorf("database query failed: %w", err)'

  bad:
    - 'return err  // No context!'
    - 'return fmt.Errorf("error: %v", err)  // Loses error chain'
    - 'result, _ := doSomething()  // Swallowed error!'
```

### CLI Output Format

```yaml
cli_output:
  success:
    # Plain output for scripting/piping
    # Use structured formats when --json flag is provided

  error:
    # Errors go to stderr
    # Include actionable context

  status_codes:
    success: 0
    user_error: 1
    internal_error: 2
```

### Database Patterns (SQLite)

```yaml
database:
  transactions:
    rule: Use transactions for multi-step operations
    pattern: |
      tx, err := db.Begin()
      if err != nil {
          return err
      }
      defer tx.Rollback()

      // ... operations ...

      return tx.Commit()

  queries:
    prefer: "Prepared statements"
    avoid: "String concatenation (SQL injection risk)"
```

### Testing Patterns

```yaml
testing:
  style: "Table-driven tests preferred"

  pattern: |
    func TestFunction(t *testing.T) {
        tests := []struct {
            name    string
            input   InputType
            want    OutputType
            wantErr bool
        }{
            {
                name:  "valid input",
                input: validInput,
                want:  expectedOutput,
            },
            {
                name:    "invalid input returns error",
                input:   invalidInput,
                wantErr: true,
            },
        }

        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                got, err := Function(tt.input)
                if tt.wantErr {
                    require.Error(t, err)
                    return
                }
                require.NoError(t, err)
                assert.Equal(t, tt.want, got)
            })
        }
    }

  ucdd_pattern: |
    // Maps to usecases/records.yaml UC-REC-001
    func TestUC_REC_001_AddRecord(t *testing.T) {
        t.Run("AC-01: add record with primary value", func(t *testing.T) {
            // Given: ...
            // When: ...
            // Then: ...
        })
    }

  coverage:
    target: "High coverage on business logic"
    command: "go test -cover ./..."
    report: "go test -coverprofile=coverage.out ./..."

  requirements:
    - "New functionality MUST have tests"
    - "Bug fixes SHOULD have regression tests"
    - "Test both success and error paths"
    - "Follow UCDD test naming convention"
```

### Struct & Interface Design

```yaml
design:
  interfaces:
    rule: "Accept interfaces, return structs"
    location: "Define interfaces where used, not implemented"

  struct_initialization:
    prefer: "Explicit field names"
    pattern: |
      record := Record{
          ID:    id,
          Name:  name,
          Value: value,
      }
    avoid: |
      record := Record{id, name, value}  // Positional - fragile
```

---

## Go Module Conventions

```yaml
modules:
  internal:
    purpose: "Packages only importable within module"
    location: "internal/"

  imports:
    rule: "Use full module path, even for same-module imports"

  dependencies:
    add: "go get <package>@<version>"
    tidy: "go mod tidy"
    verify: "go mod verify"
```

---

## Anti-patterns

```go
// WRONG: Swallowing errors
result, _ := doSomething()

// WRONG: No error context
if err != nil {
    return err  // What failed? Where?
}

// CORRECT: Wrap with context
if err != nil {
    return fmt.Errorf("failed to fetch record %s: %w", id, err)
}

// WRONG: Hardcoded paths
db, _ := sql.Open("sqlite3", "/home/user/.stash/data.db")

// CORRECT: Configuration
db, _ := sql.Open("sqlite3", config.DatabasePath)

// WRONG: Panic in library code
func GetRecord(id string) Record {
    record, err := repo.Get(id)
    if err != nil {
        panic(err)  // Crashes the whole program!
    }
    return record
}

// CORRECT: Return error
func GetRecord(id string) (Record, error) {
    return repo.Get(id)
}

// WRONG: Empty interface without type assertion
func Process(data interface{}) {
    str := data.(string)  // Panics if not string!
}

// CORRECT: Type assertion with check
func Process(data interface{}) error {
    str, ok := data.(string)
    if !ok {
        return fmt.Errorf("expected string, got %T", data)
    }
    // ...
}
```

---

## Commands

```bash
# Build
go build ./...
go build -o stash ./cmd/stash

# Test
go test ./...
go test -v ./path/to/package/...
go test -cover ./...
go test -race ./...  # Detect race conditions

# Lint
golangci-lint run
go vet ./...

# Format
gofmt -w .
goimports -w .

# Dependencies
go mod tidy
go mod verify
go get -u ./...  # Update dependencies

# Generate
go generate ./...
```

---

## Receiving Fix Beads

When debugger hands off a fix bead:

```bash
# 1. Review the investigation
bd show <fix-bead-id>
# Check linked investigation bead for root cause

# 2. Claim the work
bd update <fix-bead-id> --status=in_progress

# 3. Implement the fix
# - Address root cause, not just symptom
# - Write regression test

# 4. Run tests
go test ./...

# 5. Commit with bead reference
git commit -m "fix(component): description [st-xxx]"

# 6. Close bead
bd close <fix-bead-id> --reason "commit abc123: fixed by <description>"
```

---

## Checklist

Before completing work:

- [ ] Bead exists and is in_progress
- [ ] Code follows existing patterns in repo
- [ ] Errors wrapped with context
- [ ] **Tests written** for new/changed functionality
- [ ] **Tests pass** (`go test ./...`)
- [ ] No lint errors (`golangci-lint run` or `go vet ./...`)
- [ ] Commit references bead ID
- [ ] Bead closed with commit hash
