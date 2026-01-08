package context

import "errors"

// Context holds the resolved runtime context for stash CLI commands.
type Context struct {
	Actor    string // Resolved actor name
	Branch   string // Current git branch (may be empty)
	StashDir string // Path to .stash directory (may be empty)
	Stash    string // Default or selected stash name (may be empty)
}

// ErrNoStashDir is returned when no .stash directory is found
var ErrNoStashDir = errors.New("no .stash directory found")

// ErrNoStash is returned when no stash is selected and none can be auto-detected
var ErrNoStash = errors.New("no stash specified and multiple stashes exist (use --stash)")

// Resolve builds full context from flags and environment.
// It resolves actor, detects git branch, finds stash directory,
// and determines the default stash.
//
// Parameters:
//   - actorFlag: value of --actor flag (empty if not provided)
//   - stashFlag: value of --stash flag (empty if not provided)
//
// Returns an error if the stash directory is required but not found.
func Resolve(actorFlag, stashFlag string) (*Context, error) {
	ctx := &Context{
		Actor:    ResolveActor(actorFlag),
		Branch:   DetectBranch(),
		StashDir: FindStashDir(),
	}

	// Resolve stash name
	if stashFlag != "" {
		ctx.Stash = stashFlag
	} else {
		ctx.Stash = DefaultStash(ctx.StashDir)
	}

	return ctx, nil
}

// ResolveRequired is like Resolve but returns an error if:
// - No .stash directory is found
// - No stash can be determined (multiple stashes exist without --stash flag)
func ResolveRequired(actorFlag, stashFlag string) (*Context, error) {
	ctx, err := Resolve(actorFlag, stashFlag)
	if err != nil {
		return nil, err
	}

	if ctx.StashDir == "" {
		return nil, ErrNoStashDir
	}

	if ctx.Stash == "" {
		return nil, ErrNoStash
	}

	return ctx, nil
}

// StashPath returns the full path to the selected stash directory.
// Returns empty string if StashDir or Stash is empty.
func (c *Context) StashPath() string {
	if c.StashDir == "" || c.Stash == "" {
		return ""
	}
	return c.StashDir + "/" + c.Stash
}
