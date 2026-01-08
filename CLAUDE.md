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
