<!-- dev-setup-template: claude/commands/pickup.md | version: 1.0.0 -->
# Session Pickup

Resume work from a previous session using a handoff bead.

## Instructions

### Step 1: Find Handoff Beads

Look for open handoff beads on the current branch:

```bash
BRANCH=$(git rev-parse --abbrev-ref HEAD)
bd list --label handoff --label $BRANCH --status open
bd list --label handoff --label $BRANCH --status in_progress
```

If no handoffs found for current branch, check all open handoffs:

```bash
bd list --label handoff --status open
bd list --label handoff --status in_progress
```

### Step 2: Display Handoff Context

For each handoff found, show the details:

```bash
bd show $HANDOFF_BEAD_ID
```

Present a summary:

```
═══════════════════════════════════════════════════════════════
 HANDOFF FOUND
═══════════════════════════════════════════════════════════════

 Handoff Bead:    $BEAD_ID
 Title:           $TITLE
 Branch:          $BRANCH_NAME
 Created:         $CREATED_DATE

 ## Session Goal
 $GOAL_FROM_DESCRIPTION

 ## Remaining Work
 $REMAINING_FROM_DESCRIPTION

 ## Related Beads
 $RELATED_BEADS

═══════════════════════════════════════════════════════════════
```

### Step 3: Mark as In Progress

Update the handoff bead to show work has resumed:

```bash
bd update $HANDOFF_BEAD_ID --status in_progress
```

### Step 4: Ask What to Do

Use AskUserQuestion to ask:

"What would you like to work on from this handoff?"

Options:
- Continue with remaining work
- Focus on a specific item
- Something else (will ask for details)

### Step 5: Load Context

Based on the handoff, load relevant files:
- Read key files mentioned in the handoff
- Check status of related beads
- Review recent git history if relevant

Then summarize what you've loaded and ask how to proceed.
