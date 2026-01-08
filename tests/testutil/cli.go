// Package testutil provides test helper functions for UCDD tests in the stash CLI project.
package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Result holds the output of a stash command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// stashBinary returns the path to the stash binary.
// It looks for the binary in the project root directory.
func stashBinary() string {
	// Get the directory of this source file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "stash"
	}

	// Navigate from tests/testutil to project root
	testutilDir := filepath.Dir(filename)
	projectRoot := filepath.Dir(filepath.Dir(testutilDir))

	// Look for stash binary in project root
	binary := filepath.Join(projectRoot, "stash")
	if _, err := os.Stat(binary); err == nil {
		return binary
	}

	// Fall back to stash in PATH
	return "stash"
}

// RunStash executes the stash CLI with given args and returns the result.
// Example: RunStash(t, "init", "inventory", "--prefix", "inv-")
func RunStash(t *testing.T, args ...string) Result {
	t.Helper()
	return RunStashInDir(t, "", args...)
}

// RunStashInDir executes stash CLI in a specific directory.
// If dir is empty, uses the current working directory.
func RunStashInDir(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	binary := stashBinary()
	cmd := exec.Command(binary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if dir != "" {
		cmd.Dir = dir
	}

	// Run command and capture exit code
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to run (e.g., binary not found)
			t.Fatalf("failed to run stash: %v", err)
		}
	}

	return Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// MustSucceed calls RunStash and fails if exit code != 0.
func MustSucceed(t *testing.T, args ...string) Result {
	t.Helper()
	return MustSucceedInDir(t, "", args...)
}

// MustSucceedInDir calls RunStashInDir and fails if exit code != 0.
func MustSucceedInDir(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	result := RunStashInDir(t, dir, args...)
	if result.ExitCode != 0 {
		t.Fatalf("expected stash to succeed, but got exit code %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	return result
}

// MustFail calls RunStash and fails if exit code == 0.
func MustFail(t *testing.T, args ...string) Result {
	t.Helper()
	return MustFailInDir(t, "", args...)
}

// MustFailInDir calls RunStashInDir and fails if exit code == 0.
func MustFailInDir(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	result := RunStashInDir(t, dir, args...)
	if result.ExitCode == 0 {
		t.Fatalf("expected stash to fail, but it succeeded\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
	return result
}
