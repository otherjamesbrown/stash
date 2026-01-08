package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssertExitCode(t *testing.T) {
	result := Result{ExitCode: 0, Stdout: "output", Stderr: ""}

	// This should not panic/fail
	AssertExitCode(t, result, 0)
}

func TestAssertContains(t *testing.T) {
	result := Result{
		ExitCode: 0,
		Stdout:   "Hello, World! This is a test.",
		Stderr:   "",
	}

	// These should not panic/fail
	AssertContains(t, result, "Hello")
	AssertContains(t, result, "World")
	AssertContains(t, result, "test")
}

func TestAssertStderrContains(t *testing.T) {
	result := Result{
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "Error: something went wrong",
	}

	// This should not panic/fail
	AssertStderrContains(t, result, "Error")
	AssertStderrContains(t, result, "wrong")
}

func TestAssertNotContains(t *testing.T) {
	result := Result{
		ExitCode: 0,
		Stdout:   "Hello, World!",
		Stderr:   "",
	}

	// This should not panic/fail
	AssertNotContains(t, result, "Goodbye")
	AssertNotContains(t, result, "error")
}

func TestAssertEmpty(t *testing.T) {
	result := Result{
		ExitCode: 0,
		Stdout:   "",
		Stderr:   "",
	}

	// This should not panic/fail
	AssertEmpty(t, result)
}

func TestAssertNotEmpty(t *testing.T) {
	result := Result{
		ExitCode: 0,
		Stdout:   "some output",
		Stderr:   "",
	}

	// This should not panic/fail
	AssertNotEmpty(t, result)
}

func TestAssertFileExists_Integration(t *testing.T) {
	tmpDir := TempDir(t)
	path := WriteFile(t, tmpDir, "test.txt", "content")

	// This should not panic/fail
	AssertFileExists(t, path)
}

func TestAssertFileNotExists_Integration(t *testing.T) {
	tmpDir := TempDir(t)

	// This should not panic/fail
	AssertFileNotExists(t, tmpDir+"/nonexistent.txt")
}

func TestAssertDirExists_Integration(t *testing.T) {
	tmpDir := TempDir(t)

	// This should not panic/fail
	AssertDirExists(t, tmpDir)
}

func TestAssertDirNotExists_Integration(t *testing.T) {
	tmpDir := TempDir(t)

	// This should not panic/fail
	AssertDirNotExists(t, tmpDir+"/nonexistent")
}

// TestResultStruct verifies the Result struct fields
func TestResultStruct(t *testing.T) {
	result := Result{
		Stdout:   "standard output",
		Stderr:   "standard error",
		ExitCode: 42,
	}

	assert.Equal(t, "standard output", result.Stdout)
	assert.Equal(t, "standard error", result.Stderr)
	assert.Equal(t, 42, result.ExitCode)
}
