# stash - Claude Configuration

This document provides Claude-specific configuration for the stash repository.

## Issue Tracking with Beads

**IMPORTANT**: We use beads for issue tracking. All tasks, bugs, and features are tracked as beads issues.

**Common beads commands:**
```bash
bd list --status open              # List open issues
bd list --priority 1               # List high priority issues
bd show <issue-id>                 # Show issue details
bd create "Title" --type feature   # Create new issue
bd update <issue-id> --status in_progress  # Update status
bd close <issue-id>                # Close an issue
bd ready                           # Find tasks with no blockers
```

**Bead ID shorthand:**
- Prefix `st-` can be omitted when referencing beads
- Example: "abc123" → st-abc123

**Session management:**
- Use `/handoff` before ending a session to preserve context
- Use `/pickup` to resume from a previous handoff
- Use `bd sync --from-main` before committing (on ephemeral branches)

## Workspace System

This project uses git worktrees for parallel development:

```bash
workspace                    # List all workspaces
workspace <name>             # Enter workspace by name or number
workspace new <name> [base]  # Create new workspace
workspace remove <name>      # Remove workspace
```

**Key paths:**
- Main repository: `~/stash`
- Worktrees: `~/worktrees/<name>`
- Beads database: Shared via BEADS_DIR environment variable

## Git-Crypt

Sensitive files are encrypted with git-crypt.

- **Key location**: `~/.config/git-crypt/stash-key`
- The workspace function auto-unlocks when creating new worktrees
- Manual unlock: `git-crypt unlock ~/.config/git-crypt/stash-key`

## Branch Strategy

All development happens on feature branches merged directly to `main`.

## Session Workflows

### Starting a Session
```bash
bd ready                     # Find unblocked work
bd list --status=in_progress # Check ongoing work
workspaces                   # See active worktrees
```

### Creating a Feature Branch
```bash
workspace new feature-xyz main  # Create worktree from main
```

### Ending a Session
```bash
/handoff                     # Create handoff bead
bd sync --from-main          # Pull beads updates (ephemeral branches)
git add . && git commit      # Commit changes
```

## Project-Specific Notes

<!-- Add project-specific rules and patterns here -->
