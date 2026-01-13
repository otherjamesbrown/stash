// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/storage"
)

var (
	migrateDryRun  bool
	migrateInspect bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Run database migrations to update the schema to the current version.

The migrate command checks if the database schema needs to be updated and
applies any necessary migrations. Migrations are idempotent and safe to run
multiple times.

Flags:
  --dry-run     Show what would be done without making changes
  --inspect     Show detailed migration plan (for AI analysis)

Examples:
  stash migrate                  # Run migrations
  stash migrate --dry-run        # Preview changes
  stash migrate --inspect        # Detailed analysis

AI Agent Examples:
  # Check if migrations needed before running
  status=$(stash migrate --inspect --json)
  if echo "$status" | jq -e '.migrations_needed' >/dev/null; then
    stash migrate
  fi

  # Safe migration workflow
  stash migrate --dry-run --json && stash migrate

Exit Codes:
  0  Success (or no migrations needed)
  1  No .stash directory found
  2  Migration failed

JSON Output (--json):
  {
    "current_schema": 1,
    "target_schema": 2,
    "migrations_needed": true,
    "migrations": ["add_validation_columns"]
  }`,
	Args: cobra.NoArgs,
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show what would be done without making changes")
	migrateCmd.Flags().BoolVar(&migrateInspect, "inspect", false, "Show detailed migration plan")
	rootCmd.AddCommand(migrateCmd)
}

// Migration represents a database migration.
type Migration struct {
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// migrations is the list of all migrations in order.
// Add new migrations here when schema changes are needed.
var migrations = []Migration{
	{
		Version:     1,
		Name:        "initial_schema",
		Description: "Initial schema with _stash_meta table",
	},
	// Future migrations go here:
	// {
	//     Version:     2,
	//     Name:        "add_validation_columns",
	//     Description: "Add validation columns to schema",
	// },
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Find .stash directory
	ctx, err := context.Resolve(GetActorName(), "")
	if err != nil || ctx.StashDir == "" {
		ExitNoStashDir()
		return nil
	}

	// Load metadata to get current schema version
	metadata, err := loadStashMetadata(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	currentSchema := metadata.SchemaVersion
	if currentSchema == 0 {
		// Legacy: schema version not set, assume version 1
		currentSchema = 1
	}

	targetSchema := CurrentSchemaVersion
	migrationsNeeded := currentSchema < targetSchema

	// Find pending migrations
	var pendingMigrations []Migration
	for _, m := range migrations {
		if m.Version > currentSchema && m.Version <= targetSchema {
			pendingMigrations = append(pendingMigrations, m)
		}
	}

	// Inspect mode: just show status
	if migrateInspect || migrateDryRun {
		if GetJSONOutput() {
			migrationNames := make([]string, len(pendingMigrations))
			for i, m := range pendingMigrations {
				migrationNames[i] = m.Name
			}
			output := map[string]interface{}{
				"current_schema":    currentSchema,
				"target_schema":     targetSchema,
				"migrations_needed": migrationsNeeded,
				"migrations":        migrationNames,
				"stash_dir":         ctx.StashDir,
			}
			data, _ := json.Marshal(output)
			fmt.Println(string(data))
		} else if !IsQuiet() {
			fmt.Printf("Current schema version: %d\n", currentSchema)
			fmt.Printf("Target schema version: %d\n", targetSchema)
			fmt.Println()

			if !migrationsNeeded {
				fmt.Println("No migrations needed. Database is up to date.")
			} else {
				fmt.Println("Pending migrations:")
				for _, m := range pendingMigrations {
					fmt.Printf("  %d. %s - %s\n", m.Version, m.Name, m.Description)
				}
				if migrateDryRun {
					fmt.Println("\n(dry-run mode, no changes made)")
				}
			}
		}
		return nil
	}

	// No migrations needed
	if !migrationsNeeded {
		if GetJSONOutput() {
			output := map[string]interface{}{
				"migrated":       false,
				"schema_version": currentSchema,
				"message":        "Database is up to date",
			}
			data, _ := json.Marshal(output)
			fmt.Println(string(data))
		} else if !IsQuiet() {
			fmt.Println("Database is up to date. No migrations needed.")
		}
		return nil
	}

	// Run migrations
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer store.Close()

	appliedMigrations := []string{}
	for _, m := range pendingMigrations {
		if !IsQuiet() && !GetJSONOutput() {
			fmt.Printf("Running migration %d: %s...\n", m.Version, m.Name)
		}

		// Run the migration
		if err := runMigrationByVersion(store, m.Version); err != nil {
			fmt.Fprintf(os.Stderr, "Error: migration %d failed: %v\n", m.Version, err)
			Exit(2)
			return nil
		}

		appliedMigrations = append(appliedMigrations, m.Name)
	}

	// Update schema version in metadata
	metadata.SchemaVersion = targetSchema
	if err := saveStashMetadata(ctx.StashDir, metadata); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if GetJSONOutput() {
		output := map[string]interface{}{
			"migrated":       true,
			"schema_version": targetSchema,
			"applied":        appliedMigrations,
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	} else if !IsQuiet() {
		fmt.Printf("\nMigrations complete. Schema version: %d\n", targetSchema)
	}

	return nil
}

// runMigrationByVersion runs a specific migration.
// Add migration logic here as new migrations are needed.
func runMigrationByVersion(store *storage.Store, version int) error {
	switch version {
	case 1:
		// Initial schema - nothing to do, tables are created automatically
		return nil

	// Future migrations:
	// case 2:
	//     return migrateV2AddValidationColumns(store)

	default:
		return fmt.Errorf("unknown migration version: %d", version)
	}
}

// resetMigrateFlags resets migrate command flags for testing.
func resetMigrateFlags() {
	migrateDryRun = false
	migrateInspect = false
}
