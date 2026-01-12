<!-- dev-setup-template: claude/CLAUDE.md.template | version: 1.0.0 -->
# stash

A record-centric structured data store for AI agents. Fluid schema, hierarchical records, dual storage (JSONL + SQLite).

## Key Documents

| Document | Purpose |
|----------|---------|
| **[ARCHITECTURE.md](./ARCHITECTURE.md)** | System design, components, principles, docs map |
| **[context/agents.md](./context/agents.md)** | Core agent rules (NEVER/ALWAYS, domains, handoffs) |
| **[context/beads.md](./context/beads.md)** | Beads workflow, commands, templates |

> **Navigation**: CLAUDE.md is the entry point. For system architecture, see ARCHITECTURE.md. For agent rules, see context/agents.md.

## AI-Coding First Principles

| Principle | Why |
|-----------|-----|
| **Use-case driven** | Specs drift from implementation. Tests anchored to use-cases catch this. |
| **High test coverage** | AI generates code fast; tests are our safety net. |
| **Defined agents** | Consistent agent patterns prevent context bloat and ensure reproducibility. |
| **Debugger agent** | Systematic debugging, not random edits. Always understand root cause first. |
| **Context hygiene** | Watch for bloat. Summarize, close beads, use /handoff. |

## Beads Issue Tracking

**Prefix**: `st-`

> When user refers to `st-xxx`, it's a bead ID - use `bd show st-xxx` to view it.

```bash
bd ready                    # Find unblocked tasks
bd list --status open       # All open issues
bd update <id> --status in_progress
bd close <id> --reason "commit abc123: summary"
```

See **[context/beads.md](./context/beads.md)** for full workflow.

## Quick Commands

```bash
# Build
go build ./...

# Test
go test ./...
```

## Session Workflows

```bash
# Start
bd ready                     # Find unblocked work
workspace                    # List workspaces

# End - MANDATORY (work not done until pushed)
git status                   # Check what changed
git add <files>              # Stage changes
bd sync                      # Commit beads
git commit -m "..."          # Commit code
git push                     # PUSH TO REMOTE - required!
git status                   # Must show "up to date"
```

**CRITICAL**: Never stop before pushing. Never say "ready to push when you are" - YOU must push.

## Branch Strategy

All development happens on feature branches merged directly to `main`.

## Workspace

```bash
workspace                    # List workspaces
workspace <name>             # Enter workspace
workspace new <name> [base]  # Create new workspace
workspace remove <name>      # Remove workspace
```

- Main repo: `~/stash`
- Worktrees: `~/worktrees/<name>`

## Development Discipline (UCDD)

This project uses **Use Case Driven Development**. All features are specified in `usecases/*.yaml` before implementation.

1. **Read the use case file** before implementing
2. **Write failing tests first** (one subtest per AC)
3. **Implement until tests pass**
4. **STOP** - Do not add anything not in acceptance criteria

| File | Prefix | Feature Area |
|------|--------|--------------|
| `usecases/stash.yaml` | UC-ST- | Stash management |
| `usecases/columns.yaml` | UC-COL- | Column management |
| `usecases/records.yaml` | UC-REC- | Record operations |
| `usecases/query.yaml` | UC-QRY- | Querying |
| `usecases/import.yaml` | UC-IMP- | Import/export |
| `usecases/sync.yaml` | UC-SYN-, UC-DMN- | Sync, daemon |

## Help Text Standards

Stash is **agent-native** - help text must serve both humans AND AI agents. When adding/updating commands:

### Required Help Elements

1. **AI Agent Examples** - Every command needs automation-friendly examples:
   ```
   Examples:
     stash add "Laptop" --set Price=999           # Human example

   AI Agent Examples:
     # Batch processing pattern
     stash query "SELECT id FROM inventory WHERE status IS NULL" --json | \
       jq -r '.[].id' | while read id; do stash set $id status="processing"; done
   ```

2. **JSON Output Documentation** - Show exact structure:
   ```
   JSON Output (--json):
     {"_id": "inv-xxx", "_created_by": "agent", "Name": "Laptop", "Price": 999}
   ```

3. **Error Handling** - Document failure modes for automation:
   ```
   Exit Codes:
     0 - Success
     1 - Record/resource not found
     2 - Validation error
   ```

### Help Text Checklist

When implementing a new command, help text MUST include:

- [ ] **Short description** - One line, starts with verb
- [ ] **Long description** - When to use, what it does
- [ ] **Human examples** - Basic usage patterns
- [ ] **AI agent examples** - Batch/automation patterns with `--json`
- [ ] **JSON output format** - Exact structure returned
- [ ] **Exit codes** - What each code means
- [ ] **Related commands** - What to use next

### Reference

See `feedback/help-text-feedback.md` for real user feedback on help effectiveness.
