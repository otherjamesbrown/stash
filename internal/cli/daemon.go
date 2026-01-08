package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/daemon"
)

const (
	// DefaultStashDir is the default directory for stash data.
	DefaultStashDir = ".stash"
	// DefaultLogLines is the default number of log lines to show.
	DefaultLogLines = 50
)

var (
	logLines int
	follow   bool
)

// daemonCmd represents the daemon command group.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage background sync daemon",
	Long: `Manage the background sync daemon that watches for changes
and keeps the SQLite cache synchronized with JSONL files.

The daemon runs in the background and periodically syncs changes.`,
}

// daemonStartCmd starts the daemon.
var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the background daemon",
	Long:  `Start the background sync daemon. Idempotent - no error if already running.`,
	RunE:  runDaemonStart,
}

// daemonStopCmd stops the daemon.
var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background daemon",
	Long:  `Stop the background sync daemon gracefully. Idempotent - no error if not running.`,
	RunE:  runDaemonStop,
}

// daemonRestartCmd restarts the daemon.
var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the background daemon",
	Long:  `Stop and start the background sync daemon.`,
	RunE:  runDaemonRestart,
}

// daemonStatusCmd shows daemon status.
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Show the current status of the background sync daemon.`,
	RunE:  runDaemonStatus,
}

// daemonLogsCmd shows daemon logs.
var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View daemon logs",
	Long:  `View the daemon log file. Shows recent entries by default.`,
	RunE:  runDaemonLogs,
}

// daemonRunCmd runs the daemon in foreground (internal use).
var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run daemon in foreground (internal)",
	Hidden: true,
	RunE:   runDaemonRun,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
	daemonCmd.AddCommand(daemonRunCmd)

	// Flags for logs command
	daemonLogsCmd.Flags().IntVarP(&logLines, "lines", "n", DefaultLogLines, "Number of lines to show")
	daemonLogsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (not implemented)")
}

// getStashDir returns the .stash directory path.
func getStashDir() string {
	// Try to find .stash in current directory or parents
	dir, err := os.Getwd()
	if err != nil {
		return DefaultStashDir
	}

	for {
		stashDir := filepath.Join(dir, DefaultStashDir)
		if _, err := os.Stat(stashDir); err == nil {
			return stashDir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Default to current directory
	return DefaultStashDir
}

// runDaemonStart handles the daemon start command.
func runDaemonStart(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	d := daemon.New(stashDir)

	// Check if already running
	running, pid := d.IsRunning()
	if running {
		if jsonOutput {
			output := map[string]interface{}{
				"status":  "already_running",
				"pid":     pid,
				"message": "Daemon is already running",
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(output)
		}
		fmt.Printf("Daemon is already running (PID: %d)\n", pid)
		return nil
	}

	// Ensure .stash directory exists
	if err := os.MkdirAll(stashDir, 0755); err != nil {
		return fmt.Errorf("creating stash directory: %w", err)
	}

	if err := d.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Get the new PID
	_, newPID := d.IsRunning()

	if jsonOutput {
		output := map[string]interface{}{
			"status":  "started",
			"pid":     newPID,
			"message": "Daemon started successfully",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Printf("Daemon started (PID: %d)\n", newPID)
	return nil
}

// runDaemonStop handles the daemon stop command.
func runDaemonStop(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	d := daemon.New(stashDir)

	running, pid := d.IsRunning()
	if !running {
		if jsonOutput {
			output := map[string]interface{}{
				"status":  "not_running",
				"message": "Daemon is not running",
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(output)
		}
		fmt.Println("Daemon is not running")
		return nil
	}

	if err := d.Stop(); err != nil {
		return fmt.Errorf("stopping daemon: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":      "stopped",
			"stopped_pid": pid,
			"message":     "Daemon stopped successfully",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Printf("Daemon stopped (was PID: %d)\n", pid)
	return nil
}

// runDaemonRestart handles the daemon restart command.
func runDaemonRestart(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	d := daemon.New(stashDir)

	oldRunning, oldPID := d.IsRunning()

	if err := d.Restart(); err != nil {
		return fmt.Errorf("restarting daemon: %w", err)
	}

	_, newPID := d.IsRunning()

	if jsonOutput {
		output := map[string]interface{}{
			"status":  "restarted",
			"old_pid": oldPID,
			"new_pid": newPID,
			"message": "Daemon restarted successfully",
		}
		if !oldRunning {
			output["old_pid"] = nil
			output["status"] = "started"
			output["message"] = "Daemon started (was not running)"
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	if oldRunning {
		fmt.Printf("Daemon restarted (old PID: %d, new PID: %d)\n", oldPID, newPID)
	} else {
		fmt.Printf("Daemon started (was not running, new PID: %d)\n", newPID)
	}
	return nil
}

// runDaemonStatus handles the daemon status command.
func runDaemonStatus(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	d := daemon.New(stashDir)

	status, err := d.GetStatus()
	if err != nil {
		return fmt.Errorf("getting daemon status: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	if !status.Running {
		fmt.Println("Daemon Status: not running")
		return nil
	}

	fmt.Println("Daemon Status: running")
	fmt.Printf("  PID: %d\n", status.PID)

	if status.UptimeSeconds > 0 {
		fmt.Printf("  Uptime: %s\n", formatDuration(time.Duration(status.UptimeSeconds)*time.Second))
	}

	if !status.LastSync.IsZero() {
		ago := time.Since(status.LastSync)
		fmt.Printf("  Last sync: %s ago\n", formatDuration(ago))
	}

	if status.StashesWatched > 0 {
		fmt.Printf("  Watching: %d stashes\n", status.StashesWatched)
	}

	if status.MemoryMB > 0 {
		fmt.Printf("  Memory: %.1f MB\n", status.MemoryMB)
	}

	return nil
}

// runDaemonLogs handles the daemon logs command.
func runDaemonLogs(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	d := daemon.New(stashDir)

	if !d.LogExists() {
		if jsonOutput {
			output := map[string]interface{}{
				"logs":    []string{},
				"message": "No log file found",
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(output)
		}
		fmt.Println("No log file found")
		return nil
	}

	lines, err := daemon.TailLog(d.LogFile(), logLines)
	if err != nil {
		return fmt.Errorf("reading log file: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"logs":  lines,
			"count": len(lines),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	if len(lines) == 0 {
		fmt.Println("Log file is empty")
		return nil
	}

	for _, line := range lines {
		fmt.Println(line)
	}

	return nil
}

// runDaemonRun runs the daemon in the foreground.
// This is called when the daemon is started as a background process.
func runDaemonRun(cmd *cobra.Command, args []string) error {
	stashDir := getStashDir()
	proc := daemon.NewProcess(stashDir)

	ctx := context.Background()
	return proc.Run(ctx)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
