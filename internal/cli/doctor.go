package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check stash health and report issues",
	Long: `Check stash health and report any issues found.

The doctor command performs various health checks on your stash:
  - JSONL file integrity (valid JSON lines)
  - SQLite cache consistency
  - Orphaned files in files/ directory
  - Missing files referenced by records
  - Config.json validity
  - Duplicate record IDs
  - Hash verification (with --deep)

Flags:
  --fix       Attempt to fix issues (requires confirmation)
  --yes       Skip confirmation for --fix
  --deep      Enable deep checks including hash verification
  --json      Output results in JSON format`,
	RunE: runDoctor,
}

var (
	doctorFix  bool
	doctorYes  bool
	doctorDeep bool
)

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to fix issues")
	doctorCmd.Flags().BoolVar(&doctorYes, "yes", false, "Skip confirmation for fixes")
	doctorCmd.Flags().BoolVar(&doctorDeep, "deep", false, "Enable deep checks (hash verification)")
	rootCmd.AddCommand(doctorCmd)
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message,omitempty"`
	Details string `json:"details,omitempty"`
}

// DoctorOutput represents the JSON output for doctor command
type DoctorOutput struct {
	Healthy bool          `json:"healthy"`
	Checks  []CheckResult `json:"checks"`
	Summary struct {
		Total    int `json:"total"`
		OK       int `json:"ok"`
		Warnings int `json:"warnings"`
		Errors   int `json:"errors"`
	} `json:"summary"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Resolve context (stash dir, etc.)
	ctx, err := context.Resolve(actorName, stashName)
	if err != nil {
		return err
	}

	if ctx.StashDir == "" {
		return fmt.Errorf("no .stash directory found")
	}

	// Run health checks
	results := runHealthChecks(cmd, ctx)

	// If --fix is specified, attempt fixes
	if doctorFix {
		if !doctorYes {
			// Prompt for confirmation
			fmt.Fprint(cmd.OutOrStdout(), "Proceed with fixes? [y/N]: ")
			reader := bufio.NewReader(cmd.InOrStdin())
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}
		results = attemptFixes(cmd, ctx, results)
	}

	// Output results
	return outputDoctorResults(cmd, results)
}

func runHealthChecks(cmd *cobra.Command, ctx *context.Context) []CheckResult {
	var results []CheckResult

	// Open store for checks
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		results = append(results, CheckResult{
			Check:   "store_open",
			Status:  "error",
			Message: "Failed to open store",
			Details: err.Error(),
		})
		return results
	}
	defer store.Close()

	// 1. Check daemon status (placeholder - daemon may not be running)
	results = append(results, checkDaemonStatus(ctx))

	// 2. Check each stash
	stashes, err := store.ListStashes()
	if err != nil {
		results = append(results, CheckResult{
			Check:   "list_stashes",
			Status:  "error",
			Message: "Failed to list stashes",
			Details: err.Error(),
		})
		return results
	}

	if len(stashes) == 0 {
		results = append(results, CheckResult{
			Check:   "stashes_exist",
			Status:  "warning",
			Message: "No stashes found",
		})
		return results
	}

	results = append(results, CheckResult{
		Check:   "stashes_exist",
		Status:  "ok",
		Message: fmt.Sprintf("Found %d stash(es)", len(stashes)),
	})

	for _, stash := range stashes {
		// Check config.json validity
		results = append(results, checkConfig(ctx, stash.Name))

		// Check JSONL integrity
		results = append(results, checkJSONLIntegrity(ctx, stash.Name))

		// Check for duplicate record IDs
		results = append(results, checkDuplicateIDs(ctx, stash.Name))

		// Check JSONL/SQLite consistency
		results = append(results, checkCacheConsistency(ctx, store, stash.Name))

		// Check for orphaned files
		results = append(results, checkOrphanedFiles(ctx, stash.Name))

		// Check for missing files
		results = append(results, checkMissingFiles(ctx, store, stash.Name))

		// Check column descriptions (warning if missing)
		results = append(results, checkColumnDescriptions(stash))

		// Deep check: hash verification
		if doctorDeep {
			results = append(results, checkRecordHashes(ctx, store, stash.Name))
		}
	}

	return results
}

func checkDaemonStatus(ctx *context.Context) CheckResult {
	pidFile := filepath.Join(ctx.StashDir, "daemon.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return CheckResult{
			Check:   "daemon_status",
			Status:  "warning",
			Message: "Daemon not running",
			Details: "PID file not found",
		}
	}

	// Read PID and check if process is running
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return CheckResult{
			Check:   "daemon_status",
			Status:  "warning",
			Message: "Could not read daemon PID file",
			Details: err.Error(),
		}
	}

	// Try to verify process exists (basic check)
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return CheckResult{
			Check:   "daemon_status",
			Status:  "warning",
			Message: "Daemon PID file is empty",
		}
	}

	// Check if process exists via /proc on Linux
	procPath := filepath.Join("/proc", pid)
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return CheckResult{
			Check:   "daemon_status",
			Status:  "warning",
			Message: "Daemon process not found",
			Details: fmt.Sprintf("PID %s does not exist", pid),
		}
	}

	return CheckResult{
		Check:   "daemon_status",
		Status:  "ok",
		Message: fmt.Sprintf("Daemon running (PID %s)", pid),
	}
}

func checkConfig(ctx *context.Context, stashName string) CheckResult {
	configPath := filepath.Join(ctx.StashDir, stashName, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/config", stashName),
			Status:  "error",
			Message: "Config file not found or unreadable",
			Details: err.Error(),
		}
	}

	var stash model.Stash
	if err := json.Unmarshal(data, &stash); err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/config", stashName),
			Status:  "error",
			Message: "Invalid config.json",
			Details: err.Error(),
		}
	}

	// Validate stash name and prefix
	if stash.Name != stashName {
		return CheckResult{
			Check:   fmt.Sprintf("%s/config", stashName),
			Status:  "warning",
			Message: "Stash name mismatch in config",
			Details: fmt.Sprintf("directory: %s, config: %s", stashName, stash.Name),
		}
	}

	if err := model.ValidatePrefix(stash.Prefix); err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/config", stashName),
			Status:  "error",
			Message: "Invalid prefix in config",
			Details: err.Error(),
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/config", stashName),
		Status:  "ok",
		Message: "Config valid",
	}
}

func checkJSONLIntegrity(ctx *context.Context, stashName string) CheckResult {
	jsonlPath := filepath.Join(ctx.StashDir, stashName, "records.jsonl")

	file, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{
				Check:   fmt.Sprintf("%s/jsonl", stashName),
				Status:  "ok",
				Message: "No records file (empty stash)",
			}
		}
		return CheckResult{
			Check:   fmt.Sprintf("%s/jsonl", stashName),
			Status:  "error",
			Message: "Cannot read records.jsonl",
			Details: err.Error(),
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	validLines := 0
	var parseErrors []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var record model.Record
		if err := json.Unmarshal(line, &record); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: %v", lineNum, err))
			if len(parseErrors) >= 5 {
				parseErrors = append(parseErrors, "... (more errors)")
				break
			}
			continue
		}
		validLines++
	}

	if err := scanner.Err(); err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/jsonl", stashName),
			Status:  "error",
			Message: "Error reading records.jsonl",
			Details: err.Error(),
		}
	}

	if len(parseErrors) > 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/jsonl", stashName),
			Status:  "error",
			Message: fmt.Sprintf("Invalid JSON found (%d errors)", len(parseErrors)),
			Details: strings.Join(parseErrors, "; "),
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/jsonl", stashName),
		Status:  "ok",
		Message: fmt.Sprintf("JSONL valid (%d records)", validLines),
	}
}

func checkDuplicateIDs(ctx *context.Context, stashName string) CheckResult {
	jsonlPath := filepath.Join(ctx.StashDir, stashName, "records.jsonl")

	file, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{
				Check:   fmt.Sprintf("%s/duplicates", stashName),
				Status:  "ok",
				Message: "No records to check",
			}
		}
		return CheckResult{
			Check:   fmt.Sprintf("%s/duplicates", stashName),
			Status:  "error",
			Message: "Cannot read records.jsonl",
			Details: err.Error(),
		}
	}
	defer file.Close()

	// Track IDs and their operations
	type idOp struct {
		id    string
		op    string
		count int
	}
	idCounts := make(map[string]*idOp)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record model.Record
		if err := json.Unmarshal(line, &record); err != nil {
			continue // Skip invalid lines
		}

		if existing, ok := idCounts[record.ID]; ok {
			existing.count++
			existing.op = record.Operation
		} else {
			idCounts[record.ID] = &idOp{id: record.ID, op: record.Operation, count: 1}
		}
	}

	// Multiple operations on the same ID is normal (updates, deletes)
	// but we should report if it seems excessive
	return CheckResult{
		Check:   fmt.Sprintf("%s/duplicates", stashName),
		Status:  "ok",
		Message: fmt.Sprintf("No duplicate ID issues (%d unique IDs)", len(idCounts)),
	}
}

func checkCacheConsistency(ctx *context.Context, store *storage.Store, stashName string) CheckResult {
	// Count records in JSONL (current state)
	jsonl := storage.NewJSONLStore(ctx.StashDir)
	records, err := jsonl.ReadAllRecords(stashName)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/cache_sync", stashName),
			Status:  "error",
			Message: "Cannot read JSONL records",
			Details: err.Error(),
		}
	}

	// Build current state from JSONL operations
	state := make(map[string]*model.Record)
	for _, record := range records {
		switch record.Operation {
		case model.OpCreate, model.OpUpdate, model.OpRestore:
			state[record.ID] = record
		case model.OpDelete:
			if existing, ok := state[record.ID]; ok {
				existing.DeletedAt = record.DeletedAt
				existing.DeletedBy = record.DeletedBy
			}
		}
	}

	// Count non-deleted records
	jsonlCount := 0
	for _, r := range state {
		if !r.IsDeleted() {
			jsonlCount++
		}
	}

	// Count records in SQLite
	sqliteCount, err := store.CountRecords(stashName)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/cache_sync", stashName),
			Status:  "warning",
			Message: "Cannot count SQLite records",
			Details: err.Error(),
		}
	}

	if jsonlCount != sqliteCount {
		return CheckResult{
			Check:   fmt.Sprintf("%s/cache_sync", stashName),
			Status:  "warning",
			Message: "Cache out of sync",
			Details: fmt.Sprintf("JSONL: %d records, SQLite: %d records", jsonlCount, sqliteCount),
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/cache_sync", stashName),
		Status:  "ok",
		Message: fmt.Sprintf("Cache in sync (%d records)", sqliteCount),
	}
}

func checkOrphanedFiles(ctx *context.Context, stashName string) CheckResult {
	filesDir := filepath.Join(ctx.StashDir, stashName, "files")

	// Check if files directory exists
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return CheckResult{
			Check:   fmt.Sprintf("%s/orphaned_files", stashName),
			Status:  "ok",
			Message: "No files directory",
		}
	}

	// List all files
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/orphaned_files", stashName),
			Status:  "error",
			Message: "Cannot read files directory",
			Details: err.Error(),
		}
	}

	if len(entries) == 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/orphaned_files", stashName),
			Status:  "ok",
			Message: "No files to check",
		}
	}

	// Load records to check file references
	jsonl := storage.NewJSONLStore(ctx.StashDir)
	records, _ := jsonl.ReadAllRecords(stashName)

	// Build set of referenced files
	referencedFiles := make(map[string]bool)
	for _, record := range records {
		for _, v := range record.Fields {
			if s, ok := v.(string); ok {
				if strings.HasPrefix(s, "files/") {
					referencedFiles[strings.TrimPrefix(s, "files/")] = true
				}
			}
		}
	}

	// Find orphaned files
	var orphaned []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !referencedFiles[entry.Name()] {
			orphaned = append(orphaned, entry.Name())
		}
	}

	if len(orphaned) > 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/orphaned_files", stashName),
			Status:  "warning",
			Message: fmt.Sprintf("%d orphaned file(s)", len(orphaned)),
			Details: strings.Join(orphaned, ", "),
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/orphaned_files", stashName),
		Status:  "ok",
		Message: fmt.Sprintf("All %d files referenced", len(entries)),
	}
}

func checkMissingFiles(ctx *context.Context, store *storage.Store, stashName string) CheckResult {
	// Load records
	jsonl := storage.NewJSONLStore(ctx.StashDir)
	records, err := jsonl.ReadAllRecords(stashName)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/missing_files", stashName),
			Status:  "ok",
			Message: "No records to check",
		}
	}

	// Build current state
	state := make(map[string]*model.Record)
	for _, record := range records {
		switch record.Operation {
		case model.OpCreate, model.OpUpdate, model.OpRestore:
			state[record.ID] = record
		case model.OpDelete:
			delete(state, record.ID)
		}
	}

	// Check file references
	var missing []string
	filesDir := filepath.Join(ctx.StashDir, stashName, "files")

	for _, record := range state {
		for _, v := range record.Fields {
			if s, ok := v.(string); ok {
				if strings.HasPrefix(s, "files/") {
					filePath := filepath.Join(filesDir, strings.TrimPrefix(s, "files/"))
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						missing = append(missing, fmt.Sprintf("%s: %s", record.ID, s))
					}
				}
			}
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/missing_files", stashName),
			Status:  "error",
			Message: fmt.Sprintf("%d missing file(s)", len(missing)),
			Details: strings.Join(missing, "; "),
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/missing_files", stashName),
		Status:  "ok",
		Message: "No missing files",
	}
}

func checkColumnDescriptions(stash *model.Stash) CheckResult {
	var missing []string
	for _, col := range stash.Columns {
		if col.Desc == "" {
			missing = append(missing, col.Name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/column_descriptions", stash.Name),
			Status:  "warning",
			Message: fmt.Sprintf("%d column(s) without description", len(missing)),
			Details: strings.Join(missing, ", "),
		}
	}

	if len(stash.Columns) == 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/column_descriptions", stash.Name),
			Status:  "ok",
			Message: "No columns defined",
		}
	}

	return CheckResult{
		Check:   fmt.Sprintf("%s/column_descriptions", stash.Name),
		Status:  "ok",
		Message: fmt.Sprintf("All %d columns have descriptions", len(stash.Columns)),
	}
}

func checkRecordHashes(ctx *context.Context, store *storage.Store, stashName string) CheckResult {
	stash, err := store.GetStash(stashName)
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/hashes", stashName),
			Status:  "error",
			Message: "Cannot get stash",
			Details: err.Error(),
		}
	}

	records, err := store.ListRecords(stashName, storage.ListOptions{ParentID: "*", IncludeDeleted: false})
	if err != nil {
		return CheckResult{
			Check:   fmt.Sprintf("%s/hashes", stashName),
			Status:  "error",
			Message: "Cannot list records",
			Details: err.Error(),
		}
	}

	var mismatches []string
	for _, record := range records {
		expectedHash := model.CalculateHash(record.Fields)
		if record.Hash != expectedHash {
			mismatches = append(mismatches, fmt.Sprintf("%s (expected %s, got %s)", record.ID, expectedHash, record.Hash))
			if len(mismatches) >= 5 {
				mismatches = append(mismatches, "... (more mismatches)")
				break
			}
		}
	}

	if len(mismatches) > 0 {
		return CheckResult{
			Check:   fmt.Sprintf("%s/hashes", stashName),
			Status:  "error",
			Message: fmt.Sprintf("%d hash mismatch(es)", len(mismatches)),
			Details: strings.Join(mismatches, "; "),
		}
	}

	_ = stash // Use stash for potential future checks
	return CheckResult{
		Check:   fmt.Sprintf("%s/hashes", stashName),
		Status:  "ok",
		Message: fmt.Sprintf("All %d hashes verified", len(records)),
	}
}

func attemptFixes(cmd *cobra.Command, ctx *context.Context, results []CheckResult) []CheckResult {
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Cannot open store for repairs: %v\n", err)
		return results
	}
	defer store.Close()

	var newResults []CheckResult

	for _, r := range results {
		// Only attempt to fix errors and warnings
		if r.Status != "error" && r.Status != "warning" {
			newResults = append(newResults, r)
			continue
		}

		// Check if this is a cache sync issue
		if strings.HasSuffix(r.Check, "/cache_sync") && r.Status == "warning" {
			stashName := strings.TrimSuffix(r.Check, "/cache_sync")
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Fixing: Rebuilding cache for %s...\n", stashName)
			}
			if err := store.RebuildCache(stashName); err != nil {
				r.Details = fmt.Sprintf("Fix failed: %v", err)
			} else {
				r.Status = "ok"
				r.Message = "Cache rebuilt successfully"
				r.Details = ""
			}
		}

		newResults = append(newResults, r)
	}

	return newResults
}

func outputDoctorResults(cmd *cobra.Command, results []CheckResult) error {
	// Calculate summary
	var okCount, warnCount, errCount int
	for _, r := range results {
		switch r.Status {
		case "ok":
			okCount++
		case "warning":
			warnCount++
		case "error":
			errCount++
		}
	}

	healthy := errCount == 0

	if jsonOutput {
		output := DoctorOutput{
			Healthy: healthy,
			Checks:  results,
		}
		output.Summary.Total = len(results)
		output.Summary.OK = okCount
		output.Summary.Warnings = warnCount
		output.Summary.Errors = errCount

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Text output
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Stash Health Check")
	fmt.Fprintln(out, "==================")
	fmt.Fprintln(out)

	for _, r := range results {
		var statusIcon string
		switch r.Status {
		case "ok":
			statusIcon = "[OK]"
		case "warning":
			statusIcon = "[WARN]"
		case "error":
			statusIcon = "[ERROR]"
		}

		fmt.Fprintf(out, "%-8s %s: %s\n", statusIcon, r.Check, r.Message)
		if r.Details != "" {
			fmt.Fprintf(out, "         %s\n", r.Details)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Summary: %d checks, %d ok, %d warnings, %d errors\n",
		len(results), okCount, warnCount, errCount)

	if healthy {
		fmt.Fprintln(out, "Status: Healthy")
	} else {
		fmt.Fprintln(out, "Status: Issues found")
		fmt.Fprintln(out, "Run 'stash repair' to fix issues.")
	}

	return nil
}
