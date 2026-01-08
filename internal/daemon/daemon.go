package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// DefaultPIDFile is the default name for the PID file.
	DefaultPIDFile = "daemon.pid"
	// DefaultLogFile is the default name for the log file.
	DefaultLogFile = "daemon.log"
	// DefaultStatusFile is the default name for the status file.
	DefaultStatusFile = "daemon.status"
)

// Status represents the current state of the daemon.
type Status struct {
	Running        bool      `json:"running"`
	PID            int       `json:"pid,omitempty"`
	StartTime      time.Time `json:"start_time,omitempty"`
	UptimeSeconds  int64     `json:"uptime_seconds,omitempty"`
	LastSync       time.Time `json:"last_sync,omitempty"`
	StashesWatched int       `json:"stashes_watched,omitempty"`
	MemoryMB       float64   `json:"memory_mb,omitempty"`
}

// Daemon manages the background sync daemon process.
type Daemon struct {
	baseDir    string
	pidFile    string
	logFile    string
	statusFile string
}

// New creates a new Daemon manager.
// baseDir is typically the .stash directory.
func New(baseDir string) *Daemon {
	return &Daemon{
		baseDir:    baseDir,
		pidFile:    filepath.Join(baseDir, DefaultPIDFile),
		logFile:    filepath.Join(baseDir, DefaultLogFile),
		statusFile: filepath.Join(baseDir, DefaultStatusFile),
	}
}

// BaseDir returns the base directory for the daemon.
func (d *Daemon) BaseDir() string {
	return d.baseDir
}

// PIDFile returns the path to the PID file.
func (d *Daemon) PIDFile() string {
	return d.pidFile
}

// LogFile returns the path to the log file.
func (d *Daemon) LogFile() string {
	return d.logFile
}

// StatusFile returns the path to the status file.
func (d *Daemon) StatusFile() string {
	return d.statusFile
}

// IsRunning checks if the daemon is currently running.
// Returns (running, pid).
func (d *Daemon) IsRunning() (bool, int) {
	// Clean stale PID file first
	cleaned, _ := CleanStalePID(d.pidFile)
	if cleaned {
		return false, 0
	}

	pid, err := ReadPID(d.pidFile)
	if err != nil {
		return false, 0
	}

	if !IsProcessRunning(pid) {
		// Clean up stale PID file
		_ = RemovePID(d.pidFile)
		return false, 0
	}

	return true, pid
}

// Start starts the daemon process in the background.
// Returns nil if already running (idempotent).
func (d *Daemon) Start() error {
	// Check if already running
	running, pid := d.IsRunning()
	if running {
		// Already running - idempotent success
		return nil
	}

	// Ensure base directory exists
	if err := os.MkdirAll(d.baseDir, 0755); err != nil {
		return fmt.Errorf("creating base directory: %w", err)
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	// Start the daemon process
	cmd := exec.Command(execPath, "daemon", "run")
	cmd.Dir = d.baseDir

	// Redirect stdout/stderr to log file
	logFile, err := os.OpenFile(d.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("starting daemon process: %w", err)
	}

	// Close log file - the daemon process has its own handle
	logFile.Close()

	pid = cmd.Process.Pid

	// Write PID file
	if err := WritePID(d.pidFile, pid); err != nil {
		// Try to kill the process we just started
		_ = cmd.Process.Kill()
		return fmt.Errorf("writing pid file: %w", err)
	}

	// Write initial status
	status := &Status{
		Running:   true,
		PID:       pid,
		StartTime: time.Now(),
	}
	if err := d.writeStatus(status); err != nil {
		// Non-fatal - daemon is still running
		return nil
	}

	return nil
}

// Stop stops the daemon gracefully.
// Returns nil if not running (idempotent).
func (d *Daemon) Stop() error {
	running, pid := d.IsRunning()
	if !running {
		// Not running - idempotent success
		// Clean up PID file just in case
		_ = RemovePID(d.pidFile)
		return nil
	}

	// Send SIGTERM for graceful shutdown
	process, err := os.FindProcess(pid)
	if err != nil {
		_ = RemovePID(d.pidFile)
		return nil
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might have already exited
		_ = RemovePID(d.pidFile)
		return nil
	}

	// Wait for process to exit (up to 5 seconds)
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill if not responding
		_ = process.Kill()
	}

	// Clean up PID file
	if err := RemovePID(d.pidFile); err != nil {
		return fmt.Errorf("removing pid file: %w", err)
	}

	// Clean up status file
	_ = os.Remove(d.statusFile)

	return nil
}

// Restart stops and starts the daemon.
// Returns a new PID on success.
func (d *Daemon) Restart() error {
	if err := d.Stop(); err != nil {
		return fmt.Errorf("stopping daemon: %w", err)
	}

	// Small delay to ensure clean shutdown
	time.Sleep(100 * time.Millisecond)

	if err := d.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	return nil
}

// GetStatus returns the current daemon status.
func (d *Daemon) GetStatus() (*Status, error) {
	running, pid := d.IsRunning()

	if !running {
		return &Status{Running: false}, nil
	}

	// Try to read status from file
	status, err := d.readStatus()
	if err != nil {
		// Return basic status if file not available
		return &Status{
			Running: true,
			PID:     pid,
		}, nil
	}

	// Update with current PID (in case it changed)
	status.Running = true
	status.PID = pid

	// Calculate uptime
	if !status.StartTime.IsZero() {
		status.UptimeSeconds = int64(time.Since(status.StartTime).Seconds())
	}

	// Get memory usage
	status.MemoryMB = getProcessMemory(pid)

	return status, nil
}

// readStatus reads the status from the status file.
func (d *Daemon) readStatus() (*Status, error) {
	data, err := os.ReadFile(d.statusFile)
	if err != nil {
		return nil, err
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// writeStatus writes the status to the status file.
func (d *Daemon) writeStatus(status *Status) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}

	return os.WriteFile(d.statusFile, data, 0644)
}

// UpdateStatus updates the daemon status file (called by the daemon process).
func (d *Daemon) UpdateStatus(lastSync time.Time, stashesWatched int) error {
	status, err := d.readStatus()
	if err != nil {
		// Create new status
		status = &Status{
			Running:   true,
			StartTime: time.Now(),
		}
	}

	status.LastSync = lastSync
	status.StashesWatched = stashesWatched

	return d.writeStatus(status)
}

// getProcessMemory returns the memory usage of a process in MB.
func getProcessMemory(pid int) float64 {
	// Read from /proc/[pid]/statm
	statmPath := fmt.Sprintf("/proc/%d/statm", pid)
	data, err := os.ReadFile(statmPath)
	if err != nil {
		return 0
	}

	var size, resident, shared, text, lib, data_, dt int64
	_, err = fmt.Sscanf(string(data), "%d %d %d %d %d %d %d",
		&size, &resident, &shared, &text, &lib, &data_, &dt)
	if err != nil {
		return 0
	}

	// resident is in pages, typically 4KB
	pageSize := int64(os.Getpagesize())
	memBytes := resident * pageSize
	return float64(memBytes) / (1024 * 1024)
}

// LogExists checks if the log file exists.
func (d *Daemon) LogExists() bool {
	_, err := os.Stat(d.logFile)
	return !errors.Is(err, os.ErrNotExist)
}
