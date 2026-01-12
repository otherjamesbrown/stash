<!-- dev-setup-template: context/agents.md.template | version: 1.0.0 -->
# Agent Rules

> **Last verified**: 2026-01-08 | **Commit**: b423181

---

## Critical Rules

**NEVER:**
- Start work without a bead
- Close a bead with just "Done" - include commit hash and summary
- Work outside your agent domain without creating a handoff bead
- Ship new code without tests

**ALWAYS:**
- Create or find a bead BEFORE writing code
- Update bead status: `bd update <id> --status in_progress`
- Reference bead in commits: `fix(component): description [st-xxx]`
- Close bead with details: `bd close <id> --reason "commit <hash>: <summary>"`
- Write tests for new functionality
- Run tests before closing bead

---

## Before Starting Work

```bash
# 1. Check for blockers to new work
bd list --status open --priority 0    # Any P0? Fix first!
bd list --status open --priority 1    # P1s >7 days? Address first.
bd list --status in_progress          # Already ≥3? Finish something.

# 2. Find existing bead or create new one
bd ready                    # Find unblocked tasks
bd list --status open       # All open issues
bd create --title="..." --type=task

# 3. Claim the work
bd update <id> --status in_progress
```

**Cannot start new work if:**
- Any P0 exists (fix it first)
- You have ≥3 independent work streams in_progress (finish something first)
  - Exception: Sub-beads of the same epic count as 1 stream
  - Parallel agents working on related tasks under one epic is fine
- A P1 has been open >7 days (address it)

---

## While Working

1. **Stay in your domain** - see Agent Domains below
2. **Document progress** - add comments to bead for significant findings
3. **Follow project principles** - see ARCHITECTURE.md

---

## When Done

```bash
# 1. Run tests
go test ./...

# 2. Commit with bead reference
git commit -m "fix(component): description [st-xxx]"

# 3. Close bead with commit hash
bd close <id> --reason "commit abc1234: summary of what was done"

# 4. Create handoff beads if needed
bd create --title="Handoff: description" --type=task
bd update <new-id> --add-label="agent:target-agent"
```

---

## Session Close Protocol

**CRITICAL**: Work is NOT complete until pushed to remote.

```bash
# MANDATORY before saying "done":
git status                  # Check what changed
git add <files>             # Stage changes
bd sync                     # Commit beads changes
git commit -m "..."         # Commit code
git pull --rebase           # Get any remote changes
git push                    # PUSH TO REMOTE
git status                  # MUST show "up to date with origin"
```

**Rules:**
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
- Create beads for any remaining work before ending

---

## Preventing Bead Drift

**At decision points** (when mentioning multiple beads but user picks one):

```markdown
"Before I start on <chosen-bead>: there are also:
- <other-bead-1> (P2): <description>
- <other-bead-2> (P2): <description>

Should I:
1. Fix chosen now, then address the others
2. Defer others to next session (I'll add review dates)
3. Close others as won't-fix

Which approach?"
```

**At session end** (if open beads remain):

```markdown
"Before we finish, I see X open beads from this session:
- <bead-1> (P2): <description>
- <bead-2> (P3): <description>

Should I:
1. Fix them now
2. Defer with review dates
3. Close as won't-fix

Don't want to leave orphaned beads."
```

**NEVER** leave beads mentioned but undecided.

---

## Agent Domains

| Agent | Owns | Hand Off To |
|-------|------|-------------|
| `debugger` | Bug investigation (read-only) | Domain agents (fixes) |
| `go-developer` | cmd/, internal/, Go code | debugger (complex bugs) |

**If work is outside your domain:** Create handoff bead, don't modify.

---

## Spawn Triggers

Create handoff bead and spawn agent when:

| Trigger | Spawn |
|---------|-------|
| Bug is complex or >30 min unresolved | `debugger` |
| Need root cause analysis before fix | `debugger` |

---

## Context Loading

### Main Agent
1. Read CLAUDE.md (entry point)
2. Read context/agents.md (this file)
3. Read ARCHITECTURE.md (system design)

### Sub-Agent (spawned)
1. Read context/agents.md (always)
2. Read context/<domain>/agents.md (if exists)
3. Read relevant ARCHITECTURE.md sections

---

## Completion Checklist

Before reporting complete:
- [ ] Bead exists and is in_progress
- [ ] Tests written for new functionality
- [ ] Tests pass
- [ ] Commits reference bead ID
- [ ] Bead closed with commit hash and summary
- [ ] Handoff beads created if needed

---

## Report Format

When completing work, report:

```markdown
**Bead**: st-xxx (closed)

**Summary**: What was accomplished

**Commits**: `abc1234`: description [st-xxx]

**Files Changed**: path/to/file.go

**Tests**: Added/updated (or "N/A - no new functionality")

**Handoffs**: Beads created or "None"
```

---

## Reference Documents

| What | Where |
|------|-------|
| System architecture | ARCHITECTURE.md |
| Beads workflow | context/beads.md |
| Agent-specific context | context/<agent>/agents.md |
