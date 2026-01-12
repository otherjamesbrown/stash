<!-- dev-setup-template: context/beads.md.template | version: 1.0.0 -->
# Beads Workflow

> **Prefix**: `st-` | All work tracked as beads issues

---

## Core Rules

**NEVER:**
- Start work without a bead
- Close a bead with just "Done" - include commit hash and summary
- Work outside your agent domain without creating a handoff bead

**ALWAYS:**
- Create or find a bead BEFORE writing code
- Update status: `bd update <id> --status in_progress`
- Reference bead in commits: `fix(component): description [st-xxx]`
- Close with details: `bd close <id> --reason "commit abc123: summary"`

---

## Commands

### Finding Work

```bash
bd ready                          # Tasks with no blockers
bd list --status=open             # All open issues
bd list --status=in_progress      # Active work
bd show <id>                      # Issue details
```

### Creating Issues

```bash
bd create --title="..." --type=task --priority=2
bd create --title="..." --type=bug --priority=1
bd create --title="..." --type=feature --priority=2
```

### Priority Levels

| Priority | Use For |
|----------|---------|
| P0 | Critical - drop everything |
| P1 | High - current sprint |
| P2 | Medium - default |
| P3 | Low - backlog |
| P4 | Wishlist |

### Working on Issues

```bash
bd update <id> --status=in_progress    # Claim work
bd update <id> --add-label="component:cli"
bd comments add <id> "Progress update..."
```

### Completing Issues

```bash
# Close with reason (required)
bd close <id> --reason "commit abc123: implemented feature X"

# Close multiple at once
bd close <id1> <id2> <id3>
```

### Dependencies

```bash
bd dep add <issue> <depends-on>   # Issue depends on another
bd blocked                        # Show all blocked issues
```

---

## Workflow

### Starting a Session

```bash
bd ready                     # Find available work
bd list --status=in_progress # Check ongoing work
/pickup                      # Resume from handoff (if exists)
```

### During Work

1. **Claim the bead**: `bd update <id> --status in_progress`
2. **Stay focused**: One bead at a time when possible
3. **Document progress**: Add comments for significant findings
4. **Reference in commits**: `git commit -m "fix(cli): ... [st-xxx]"`

### Ending a Session

```bash
/handoff                     # Create handoff bead with context
bd sync --from-main          # Pull beads updates (ephemeral branches)
git add . && git commit      # Commit changes
```

---

## Handoffs

### Creating a Handoff

Use `/handoff` to preserve context across sessions. Include:
- What was the goal?
- What was completed?
- What's remaining?
- Key findings or blockers

### Resuming from Handoff

Use `/pickup` to find and load handoff context.

---

## Labels

```bash
# Component labels
bd update <id> --add-label="component:cli"
bd update <id> --add-label="component:storage"

# Agent labels (for handoffs)
bd update <id> --add-label="agent:go-developer"
bd update <id> --add-label="agent:debugger"

# Status labels
bd update <id> --add-label="blocked"
bd update <id> --add-label="needs-review"

# Context tracking
bd update <id> --add-label="context-gap"      # Bug caused by missing context
bd update <id> --add-label="context-update"   # Task to update context docs

# Use case tracking
bd update <id> --add-label="uc:UC-REC-001"    # Link to use case
bd update <id> --add-label="ac-gap"           # AC was missing
```

---

## Bead Templates

### Bug Report

```bash
bd create --title="Bug: [component] brief description" --type=bug --priority=1
bd comments add <id> "
**Symptom**: What's happening

**Expected**: What should happen

**Reproduction**:
1. Step one
2. Step two

**Environment**: development/staging/production
"
```

### Feature Task

```bash
bd create --title="Feature: [component] brief description" --type=feature --priority=2
bd comments add <id> "
**Goal**: What we're building

**Acceptance Criteria**:
- [ ] Criterion 1
- [ ] Criterion 2

**Out of Scope**: What we're NOT doing
"
```

### Handoff

```bash
bd create --title="Handoff: brief context" --type=task --priority=1
bd update <id> --add-label="handoff"
bd update <id> --add-label="<branch-name>"
bd comments add <id> "
**Goal**: Original task

**Completed**:
- Item 1
- Item 2

**Remaining**:
- Item 3
- Item 4

**Key Findings**: Important discoveries

**Related Beads**: st-xxx, st-yyy
"
```

---

## Closing Format

Always close with a reason that includes:

```bash
# For implemented work
bd close <id> --reason "commit abc123: implemented X, added tests"

# For bugs
bd close <id> --reason "commit abc123: root cause was Y, fixed by Z"

# For investigations
bd close <id> --reason "INVESTIGATED: cause was X. Fix bead: st-yyy"

# For won't fix
bd close <id> --reason "WONTFIX: reason why this won't be addressed"
```

---

## Sync Commands

```bash
bd sync                      # Full sync (import + export + commit)
bd sync --from-main          # Pull beads from main (for ephemeral branches)
bd sync --status             # Check sync status
```
