package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePID(t *testing.T) {
	t.Run("writes PID to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := WritePID(pidFile, 12345)
		require.NoError(t, err)

		// Verify file contents
		data, err := os.ReadFile(pidFile)
		require.NoError(t, err)
		assert.Equal(t, "12345\n", string(data))
	})

	t.Run("overwrites existing PID file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := WritePID(pidFile, 11111)
		require.NoError(t, err)

		err = WritePID(pidFile, 22222)
		require.NoError(t, err)

		data, err := os.ReadFile(pidFile)
		require.NoError(t, err)
		assert.Equal(t, "22222\n", string(data))
	})
}

func TestReadPID(t *testing.T) {
	t.Run("reads valid PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("12345\n"), 0644)
		require.NoError(t, err)

		pid, err := ReadPID(pidFile)
		require.NoError(t, err)
		assert.Equal(t, 12345, pid)
	})

	t.Run("reads PID without newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("12345"), 0644)
		require.NoError(t, err)

		pid, err := ReadPID(pidFile)
		require.NoError(t, err)
		assert.Equal(t, 12345, pid)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "nonexistent.pid")

		_, err := ReadPID(pidFile)
		assert.ErrorIs(t, err, ErrPIDFileNotFound)
	})

	t.Run("returns error for empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte(""), 0644)
		require.NoError(t, err)

		_, err = ReadPID(pidFile)
		assert.ErrorIs(t, err, ErrInvalidPID)
	})

	t.Run("returns error for non-numeric content", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
		require.NoError(t, err)

		_, err = ReadPID(pidFile)
		assert.ErrorIs(t, err, ErrInvalidPID)
	})

	t.Run("returns error for negative PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("-1"), 0644)
		require.NoError(t, err)

		_, err = ReadPID(pidFile)
		assert.ErrorIs(t, err, ErrInvalidPID)
	})

	t.Run("returns error for zero PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("0"), 0644)
		require.NoError(t, err)

		_, err = ReadPID(pidFile)
		assert.ErrorIs(t, err, ErrInvalidPID)
	})
}

func TestRemovePID(t *testing.T) {
	t.Run("removes existing PID file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("12345"), 0644)
		require.NoError(t, err)

		err = RemovePID(pidFile)
		require.NoError(t, err)

		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("no error for non-existent file (idempotent)", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "nonexistent.pid")

		err := RemovePID(pidFile)
		assert.NoError(t, err)
	})
}

func TestIsProcessRunning(t *testing.T) {
	t.Run("returns true for current process", func(t *testing.T) {
		pid := os.Getpid()
		assert.True(t, IsProcessRunning(pid))
	})

	t.Run("returns false for invalid PID", func(t *testing.T) {
		assert.False(t, IsProcessRunning(-1))
		assert.False(t, IsProcessRunning(0))
	})

	t.Run("returns false for non-existent PID", func(t *testing.T) {
		// Use a very high PID that's unlikely to exist
		assert.False(t, IsProcessRunning(999999999))
	})
}

func TestCleanStalePID(t *testing.T) {
	t.Run("does nothing if file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "nonexistent.pid")

		cleaned, err := CleanStalePID(pidFile)
		require.NoError(t, err)
		assert.False(t, cleaned)
	})

	t.Run("cleans invalid PID file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidFile, []byte("invalid"), 0644)
		require.NoError(t, err)

		cleaned, err := CleanStalePID(pidFile)
		require.NoError(t, err)
		assert.True(t, cleaned)

		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("cleans stale PID (process not running)", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		// Use a PID that's unlikely to exist
		err := os.WriteFile(pidFile, []byte("999999999"), 0644)
		require.NoError(t, err)

		cleaned, err := CleanStalePID(pidFile)
		require.NoError(t, err)
		assert.True(t, cleaned)

		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("does not clean valid PID (process running)", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "test.pid")

		// Use our own PID which is always running
		myPID := os.Getpid()
		err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", myPID)), 0644)
		require.NoError(t, err)

		cleaned, err := CleanStalePID(pidFile)
		require.NoError(t, err)
		assert.False(t, cleaned)

		// File should still exist
		_, err = os.Stat(pidFile)
		assert.NoError(t, err)
	})
}
