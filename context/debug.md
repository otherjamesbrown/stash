# Debug & Observability Configuration

> Project-specific debugging resources. Used by debugger agent and developers.

---

## Log Locations

```yaml
logs:
  application:
    # stash is a CLI tool - logs go to stderr
    command: "./stash <command> 2>&1"

  path: "stderr (CLI tool)"
```

---

## Debug Commands

```yaml
debug_commands:
  # Test commands
  run_tests: "go test ./..."
  run_single_test: "go test -v -run <TestName> ./..."
  test_coverage: "go test -cover ./..."
  test_race: "go test -race ./..."
  test_verbose: "go test -v ./..."

  # Build/compile check
  build_check: "go build ./..."
  build_binary: "go build -o stash ./cmd/stash"

  # Lint
  lint: "golangci-lint run"
  vet: "go vet ./..."

  # CLI testing
  cli_help: "./stash --help"
  cli_version: "./stash version"

  # Database inspection (SQLite)
  db_location: ".beads/beads.db or ~/.stash/*.db"
  db_shell: "sqlite3 <db-path>"
```

---

## Common Error Patterns

```yaml
error_patterns:
  generic:
    - "error"
    - "Error"
    - "ERROR"
    - "panic"
    - "fatal"
    - "failed"

  stash_specific:
    - "stash not found"
    - "column not found"
    - "record not found"
    - "duplicate"
    - "constraint"
    - "invalid"
```

---

## Environment-Specific Access

```yaml
environments:
  development:
    logs: "go test -v ./... 2>&1"
    shell: "go run ./cmd/stash"
    db: "sqlite3 ./test.db"

  test:
    logs: "go test -v ./... 2>&1 | tee test.log"
    coverage: "go test -coverprofile=coverage.out ./..."
```

---

## Health Checks

```yaml
health_checks:
  build:
    command: "go build ./..."

  tests:
    command: "go test ./..."

  lint:
    command: "golangci-lint run || go vet ./..."
```

---

## Quick Debug Checklist

When debugging, check these in order:

1. **Tests** - Can you reproduce with a test?
2. **Build** - Does `go build ./...` succeed?
3. **Git history** - Recent changes to affected files
4. **Config** - Environment variables, config files
5. **Database** - SQLite database state
6. **Dependencies** - `go mod tidy`, `go mod verify`
