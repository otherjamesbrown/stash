// Package daemon provides background sync daemon management for stash.
package daemon

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var (
	// ErrPIDFileNotFound indicates the PID file does not exist.
	ErrPIDFileNotFound = errors.New("pid file not found")
	// ErrInvalidPID indicates the PID file contains invalid data.
	ErrInvalidPID = errors.New("invalid pid in file")
	// ErrProcessNotRunning indicates the process is not running.
	ErrProcessNotRunning = errors.New("process not running")
)

// WritePID writes the given PID to the specified file.
func WritePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// ReadPID reads the PID from the specified file.
// Returns ErrPIDFileNotFound if the file doesn't exist.
// Returns ErrInvalidPID if the file contains invalid data.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrPIDFileNotFound
		}
		return 0, fmt.Errorf("reading pid file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, ErrInvalidPID
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, ErrInvalidPID
	}

	if pid <= 0 {
		return 0, ErrInvalidPID
	}

	return pid, nil
}

// RemovePID removes the PID file at the specified path.
// Returns nil if the file doesn't exist (idempotent).
func RemovePID(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing pid file: %w", err)
	}
	return nil
}

// IsProcessRunning checks if a process with the given PID is running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	// to check if the process actually exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CleanStalePID removes the PID file if it references a non-running process.
// Returns true if the PID file was stale and removed.
func CleanStalePID(path string) (bool, error) {
	pid, err := ReadPID(path)
	if err != nil {
		if errors.Is(err, ErrPIDFileNotFound) {
			return false, nil
		}
		// Invalid PID file - remove it
		if errors.Is(err, ErrInvalidPID) {
			if err := RemovePID(path); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	if !IsProcessRunning(pid) {
		if err := RemovePID(path); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
