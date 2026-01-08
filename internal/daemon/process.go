package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/user/stash/internal/storage"
)

const (
	// SyncInterval is the default interval between syncs.
	SyncInterval = 5 * time.Second
	// MaxLogSize is the maximum log file size before rotation (10MB).
	MaxLogSize = 10 * 1024 * 1024
	// MaxLogFiles is the number of rotated log files to keep.
	MaxLogFiles = 3
)

// Process represents a running daemon process.
type Process struct {
	daemon     *Daemon
	logger     *log.Logger
	logFile    *os.File
	stopChan   chan struct{}
	stashesDir string
	watcher    *Watcher
}

// NewProcess creates a new daemon process.
func NewProcess(baseDir string) *Process {
	d := New(baseDir)
	return &Process{
		daemon:     d,
		stopChan:   make(chan struct{}),
		stashesDir: filepath.Dir(baseDir), // Parent of .stash is where stashes are
	}
}

// Run starts the daemon process loop.
// This should be called by the background process after fork.
func (p *Process) Run(ctx context.Context) error {
	// Setup logging
	if err := p.setupLogging(); err != nil {
		return fmt.Errorf("setting up logging: %w", err)
	}
	defer p.closeLogging()

	p.logger.Println("Daemon starting...")

	// Setup file watcher
	if err := p.setupWatcher(); err != nil {
		p.logger.Printf("Warning: could not setup file watcher: %v", err)
	} else {
		defer p.watcher.Close()
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Create a ticker for periodic sync
	ticker := time.NewTicker(SyncInterval)
	defer ticker.Stop()

	// Update status
	p.updateStatus()

	p.logger.Println("Daemon started, watching for changes...")

	for {
		select {
		case <-ctx.Done():
			p.logger.Println("Context cancelled, shutting down...")
			return ctx.Err()

		case sig := <-sigChan:
			p.logger.Printf("Received signal %v, shutting down...", sig)
			return nil

		case <-p.stopChan:
			p.logger.Println("Stop requested, shutting down...")
			return nil

		case <-ticker.C:
			p.performSync()
			p.updateStatus()
			p.checkLogRotation()
		}
	}
}

// Stop signals the process to stop.
func (p *Process) Stop() {
	close(p.stopChan)
}

// setupLogging initializes the log file.
func (p *Process) setupLogging() error {
	logPath := p.daemon.LogFile()

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	p.logFile = f
	p.logger = log.New(f, "[stash-daemon] ", log.LstdFlags)

	return nil
}

// closeLogging closes the log file.
func (p *Process) closeLogging() {
	if p.logFile != nil {
		p.logFile.Close()
	}
}

// setupWatcher initializes the file watcher for auto-sync.
func (p *Process) setupWatcher() error {
	logFn := func(format string, args ...interface{}) {
		p.logger.Printf(format, args...)
	}

	watcher, err := NewWatcher(p.daemon.BaseDir(), p.rebuildStashCache, logFn)
	if err != nil {
		return err
	}

	if err := watcher.Start(); err != nil {
		watcher.Close()
		return err
	}

	p.watcher = watcher
	p.logger.Println("File watcher started")
	return nil
}

// rebuildStashCache rebuilds the SQLite cache for a stash from JSONL.
func (p *Process) rebuildStashCache(stashName string) error {
	store, err := storage.NewStore(p.daemon.BaseDir())
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer store.Close()

	if err := store.RebuildCache(stashName); err != nil {
		return fmt.Errorf("rebuilding cache: %w", err)
	}

	// Update last sync time in status
	p.updateStatus()

	return nil
}

// performSync performs the actual sync operation.
func (p *Process) performSync() {
	// This is a placeholder for actual sync logic
	// In a full implementation, this would:
	// 1. Watch for JSONL file changes
	// 2. Sync changes to SQLite cache
	// 3. Handle any pending operations

	p.logger.Println("Performing sync check...")
}

// updateStatus updates the daemon status file.
func (p *Process) updateStatus() {
	stashCount := p.countWatchedStashes()
	if err := p.daemon.UpdateStatus(time.Now(), stashCount); err != nil {
		p.logger.Printf("Error updating status: %v", err)
	}
}

// countWatchedStashes counts the number of stashes being watched.
func (p *Process) countWatchedStashes() int {
	// Use watcher if available
	if p.watcher != nil {
		return p.watcher.StashCount()
	}

	// Fallback: count stash directories manually
	entries, err := os.ReadDir(p.daemon.BaseDir())
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

// checkLogRotation checks if log rotation is needed and performs it.
func (p *Process) checkLogRotation() {
	if p.logFile == nil {
		return
	}

	info, err := p.logFile.Stat()
	if err != nil {
		return
	}

	if info.Size() < MaxLogSize {
		return
	}

	p.logger.Println("Rotating log file...")

	if err := p.rotateLog(); err != nil {
		p.logger.Printf("Error rotating log: %v", err)
	}
}

// rotateLog rotates the log file.
func (p *Process) rotateLog() error {
	logPath := p.daemon.LogFile()

	// Close current log file
	if p.logFile != nil {
		p.logFile.Close()
		p.logFile = nil
	}

	// Rotate existing log files
	for i := MaxLogFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", logPath, i)
		newPath := fmt.Sprintf("%s.%d", logPath, i+1)
		_ = os.Rename(oldPath, newPath)
	}

	// Rename current log to .1
	_ = os.Rename(logPath, fmt.Sprintf("%s.1", logPath))

	// Remove oldest log if it exists
	_ = os.Remove(fmt.Sprintf("%s.%d", logPath, MaxLogFiles+1))

	// Reopen log file
	return p.setupLogging()
}

// TailLog reads the last n lines from the log file.
func TailLog(logPath string, n int) ([]string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if stat.Size() == 0 {
		return nil, nil
	}

	// Read entire file for simplicity (log files are typically small)
	// For very large files, a more efficient approach would be needed
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Split into lines
	allLines := splitLines(string(data))

	// Filter out empty lines at the end
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	if len(allLines) == 0 {
		return nil, nil
	}

	// Return last n lines
	if len(allLines) <= n {
		return allLines, nil
	}

	return allLines[len(allLines)-n:], nil
}

// splitLines splits content into lines.
func splitLines(content string) []string {
	var lines []string
	var current string

	for _, c := range content {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}
