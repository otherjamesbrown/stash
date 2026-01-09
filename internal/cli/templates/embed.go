// Package templates provides embedded slash command templates for Claude Code integration.
package templates

import "embed"

// Commands contains the embedded slash command markdown files.
//
//go:embed commands/*.md
var Commands embed.FS
