# Use Case YAML Schema

This document defines the schema for use case specification files.

## File Structure

```yaml
feature: <string>           # Feature area name
description: |              # Multi-line description
  <description>

dependencies:               # Optional: other use case files this depends on
  - usecases/<file>.yaml

usecases:
  - id: UC-<PREFIX>-<NNN>   # Unique identifier
    title: <string>         # Short title
    interface: cli          # Interface type (cli for now)
    status: active          # active, deprecated, planned

    description: |          # What this use case accomplishes
      <description>

    actor: User             # Who performs this action

    preconditions:          # What must be true before
      - <condition>

    depends_on:             # Other use cases that must work first
      - UC-XXX-NNN

    acceptance_criteria:    # Testable criteria
      - id: AC-NN
        criterion: <short description>
        given: <precondition>
        when: <action>
        then:
          - <expected outcome>
          - <expected outcome>

    in_scope:               # What this use case covers
      - <item>

    out_of_scope:           # Explicitly not covered
      - <item>

    must_not:               # Anti-requirements (things that must NOT happen)
      - <constraint>
```

## ID Prefixes

| Prefix | Feature Area |
|--------|--------------|
| UC-ST- | Stash management (init, drop, info, onboard, prime) |
| UC-COL- | Column management (add, list, describe, rename, drop) |
| UC-REC- | Record operations (add, set, show, file, delete, restore, purge) |
| UC-QRY- | Querying (list, children, query, history) |
| UC-IMP- | Import/export |
| UC-SYN- | Sync, daemon, maintenance |
| UC-STAT- | Field statistics and analysis |

## Acceptance Criteria Format

Each AC follows Given-When-Then:

```yaml
- id: AC-01
  criterion: Brief description of what's being tested
  given: Initial state or precondition
  when: Action performed (usually a command)
  then:
    - First expected outcome
    - Second expected outcome
```

## Test Mapping

Each acceptance criterion maps to a subtest:

```go
func TestUC_ST_001_InitializeStash(t *testing.T) {
    t.Run("AC-01: create stash with required fields", func(t *testing.T) {
        // Given: ...
        // When: ...
        // Then: ...
    })
}
```

## Anti-Requirements (must_not)

The `must_not` section defines constraints that are tested separately:

```go
func TestUC_ST_001_InitializeStash_MustNot(t *testing.T) {
    t.Run("must not overwrite existing stash", func(t *testing.T) {
        // ...
    })
}
```
