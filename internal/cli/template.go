// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
)

// Template represents a saved query template
type Template struct {
	Name      string    `json:"name"`
	Query     string    `json:"query"`
	Desc      string    `json:"desc,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// Error codes for template operations
const (
	ErrCodeTemplateNotFound = "TEMPLATE_NOT_FOUND"
	ErrCodeTemplateExists   = "TEMPLATE_EXISTS"
	ErrCodeInvalidTemplate  = "INVALID_TEMPLATE"
)

var templateDesc string

// templateNameRegex validates template names: alphanumeric, hyphens, underscores
var templateNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage query templates",
	Long: `Save and run reusable query templates.

Templates let you save frequently-used queries for quick access.

Examples:
  stash template save "needs-review" "SELECT id, name FROM tasks WHERE status='pending'"
  stash template run "needs-review"
  stash template list

AI Agent Examples:
  # Save template and use in automation
  stash template save "unprocessed" "SELECT id FROM jobs WHERE processed_at IS NULL"
  stash template run "unprocessed" --json | jq -r '.[].id' | while read id; do
    process_job "$id"
  done

  # Check if template exists before using
  if stash template show "my-template" --json >/dev/null 2>&1; then
    stash template run "my-template" --json
  fi

Exit Codes:
  0  Success
  1  Template not found
  2  Validation error (invalid name, empty query)

JSON Output (--json):
  template list: [{"name": "high-priority", "query": "SELECT...", "created_at": "..."}]
  template show: {"name": "high-priority", "query": "SELECT...", "description": "..."}
  template run: (same as stash query output)
`,
}

var templateSaveCmd = &cobra.Command{
	Use:   "save <name> <query>",
	Short: "Save a new query template",
	Long: `Save a query as a reusable template.

Template names must:
  - Start with a letter
  - Contain only letters, numbers, hyphens, and underscores

The query must be a valid SELECT statement.

Examples:
  stash template save "high-priority" "SELECT * FROM inventory WHERE priority='high'"
  stash template save "needs-review" "SELECT id, name FROM tasks WHERE status='pending'" --desc "Tasks needing review"

Exit Codes:
  0  Success
  2  Validation error (invalid name, empty query, non-SELECT query)`,
	Args: cobra.ExactArgs(2),
	RunE: runTemplateSave,
}

var templateRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a saved template",
	Long: `Run a saved query template.

Output flags from 'stash query' are supported:
  --json         Output as JSON array
  --csv          Output as CSV with headers
  --no-headers   Omit header row in CSV output
  --columns      Select specific columns in CSV output

Examples:
  stash template run "high-priority"
  stash template run "needs-review" --json
  stash template run "report" --csv > report.csv

Exit Codes:
  0  Success
  1  Template not found`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateRun,
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved templates",
	Long: `List all saved query templates.

Examples:
  stash template list
  stash template list --json

Exit Codes:
  0  Success`,
	Args: cobra.NoArgs,
	RunE: runTemplateList,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show template details",
	Long: `Show details for a specific template.

Examples:
  stash template show "high-priority"
  stash template show "needs-review" --json

Exit Codes:
  0  Success
  1  Template not found`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateShow,
}

var templateRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Delete a template",
	Long: `Delete a saved query template.

Examples:
  stash template rm "high-priority"

Exit Codes:
  0  Success
  1  Template not found`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateRm,
}

func init() {
	templateSaveCmd.Flags().StringVar(&templateDesc, "desc", "", "Template description")

	// Add query-compatible flags to run command
	templateRunCmd.Flags().BoolVar(&queryCSV, "csv", false, "Output as CSV format")
	templateRunCmd.Flags().BoolVar(&queryNoHeaders, "no-headers", false, "Omit header row in CSV output")
	templateRunCmd.Flags().StringVar(&queryColumns, "columns", "", "Select specific columns in CSV output (comma-separated)")

	templateCmd.AddCommand(templateSaveCmd)
	templateCmd.AddCommand(templateRunCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateRmCmd)
	rootCmd.AddCommand(templateCmd)
}

// validateTemplateName validates that a template name is valid
func validateTemplateName(name string) error {
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("template name must be at most 64 characters")
	}
	if !templateNameRegex.MatchString(name) {
		return fmt.Errorf("template name must start with a letter and contain only letters, numbers, hyphens, and underscores")
	}
	return nil
}

// templatesFilePath returns the path to the templates file
func templatesFilePath(stashDir string) string {
	return filepath.Join(stashDir, "templates.json")
}

// loadTemplates loads all templates from the templates file
func loadTemplates(stashDir string) ([]*Template, error) {
	path := templatesFilePath(stashDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Template{}, nil
		}
		return nil, err
	}

	var templates []*Template
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// saveTemplates saves all templates to the templates file
func saveTemplates(stashDir string, templates []*Template) error {
	path := templatesFilePath(stashDir)
	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// findTemplate finds a template by name (case-sensitive)
func findTemplate(templates []*Template, name string) *Template {
	for _, t := range templates {
		if t.Name == name {
			return t
		}
	}
	return nil
}

func runTemplateSave(cmd *cobra.Command, args []string) error {
	name := args[0]
	query := args[1]

	// Validate template name
	if err := validateTemplateName(name); err != nil {
		ExitValidationError(err.Error(), map[string]interface{}{"name": name})
		return nil
	}

	// Validate query is not empty
	if query == "" {
		ExitValidationError("query cannot be empty", nil)
		return nil
	}

	// Validate query is a SELECT statement
	if !isSelectQuery(query) {
		ExitValidationError("only SELECT queries are allowed in templates", map[string]interface{}{"query": query})
		return nil
	}

	// Resolve context (just need stash dir)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Templates require a .stash directory
	if ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load existing templates
	templates, err := loadTemplates(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Check if template already exists
	if existing := findTemplate(templates, name); existing != nil {
		ExitWithError(2, ErrCodeTemplateExists,
			fmt.Sprintf("template '%s' already exists", name),
			map[string]interface{}{"name": name})
		return nil
	}

	// Create new template
	now := time.Now()
	template := &Template{
		Name:      name,
		Query:     query,
		Desc:      templateDesc,
		CreatedAt: now,
		CreatedBy: ctx.Actor,
	}
	templates = append(templates, template)

	// Save templates
	if err := saveTemplates(ctx.StashDir, templates); err != nil {
		return fmt.Errorf("failed to save templates: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"name":       template.Name,
			"query":      template.Query,
			"desc":       template.Desc,
			"created_at": template.CreatedAt.Format(time.RFC3339),
			"created_by": template.CreatedBy,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Saved template '%s'\n", template.Name)
	}

	// Reset flag for next call (important for tests)
	templateDesc = ""

	return nil
}

func runTemplateRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Resolve context (just need stash dir for templates)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Templates require a .stash directory
	if ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load templates
	templates, err := loadTemplates(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Find template
	template := findTemplate(templates, name)
	if template == nil {
		ExitWithError(1, ErrCodeTemplateNotFound,
			fmt.Sprintf("template '%s' not found", name),
			map[string]interface{}{"name": name})
		return nil
	}

	// Execute the query using runQuery
	// We need to set args for the query command
	return runQuery(cmd, []string{template.Query})
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	// Resolve context (just need stash dir for templates)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Templates require a .stash directory
	if ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load templates
	templates, err := loadTemplates(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		output := make([]map[string]interface{}, len(templates))
		for i, t := range templates {
			output[i] = map[string]interface{}{
				"name":       t.Name,
				"query":      t.Query,
				"desc":       t.Desc,
				"created_at": t.CreatedAt.Format(time.RFC3339),
				"created_by": t.CreatedBy,
			}
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		if len(templates) == 0 {
			fmt.Println("No templates saved")
		} else {
			fmt.Println("Templates:")
			for _, t := range templates {
				if t.Desc != "" {
					fmt.Printf("  %s - %s\n", t.Name, t.Desc)
				} else {
					fmt.Printf("  %s\n", t.Name)
				}
			}
		}
	}

	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Resolve context (just need stash dir for templates)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Templates require a .stash directory
	if ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load templates
	templates, err := loadTemplates(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Find template
	template := findTemplate(templates, name)
	if template == nil {
		ExitWithError(1, ErrCodeTemplateNotFound,
			fmt.Sprintf("template '%s' not found", name),
			map[string]interface{}{"name": name})
		return nil
	}

	// Output result
	if GetJSONOutput() {
		output := map[string]interface{}{
			"name":       template.Name,
			"query":      template.Query,
			"desc":       template.Desc,
			"created_at": template.CreatedAt.Format(time.RFC3339),
			"created_by": template.CreatedBy,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Name: %s\n", template.Name)
		if template.Desc != "" {
			fmt.Printf("Description: %s\n", template.Desc)
		}
		fmt.Printf("Query: %s\n", template.Query)
		fmt.Printf("Created: %s by %s\n", template.CreatedAt.Format(time.RFC3339), template.CreatedBy)
	}

	return nil
}

func runTemplateRm(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Resolve context (just need stash dir for templates)
	ctx, err := context.Resolve(GetActorName(), GetStashName())
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Templates require a .stash directory
	if ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load templates
	templates, err := loadTemplates(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Find and remove template
	found := false
	var newTemplates []*Template
	for _, t := range templates {
		if t.Name == name {
			found = true
			continue // Remove this template
		}
		newTemplates = append(newTemplates, t)
	}

	if !found {
		ExitWithError(1, ErrCodeTemplateNotFound,
			fmt.Sprintf("template '%s' not found", name),
			map[string]interface{}{"name": name})
		return nil
	}

	// Save updated templates
	if err := saveTemplates(ctx.StashDir, newTemplates); err != nil {
		return fmt.Errorf("failed to save templates: %w", err)
	}

	// Output result
	if GetJSONOutput() {
		result := map[string]interface{}{
			"deleted": true,
			"name":    name,
		}
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("Deleted template '%s'\n", name)
	}

	return nil
}
