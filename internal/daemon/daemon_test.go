package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates daemon with correct paths", func(t *testing.T) {
		baseDir := "/tmp/test-stash"
		d := New(baseDir)

		assert.Equal(t, baseDir, d.BaseDir())
		assert.Equal(t, filepath.Join(baseDir, "daemon.pid"), d.PIDFile())
		assert.Equal(t, filepath.Join(baseDir, "daemon.log"), d.LogFile())
		assert.Equal(t, filepath.Join(baseDir, "daemon.status"), d.StatusFile())
	})
}

func TestDaemon_IsRunning(t *testing.T) {
	t.Run("returns false when no PID file", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		running, pid := d.IsRunning()
		assert.False(t, running)
		assert.Equal(t, 0, pid)
	})

	t.Run("returns false when PID file has invalid content", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		err := os.WriteFile(d.PIDFile(), []byte("invalid"), 0644)
		require.NoError(t, err)

		running, pid := d.IsRunning()
		assert.False(t, running)
		assert.Equal(t, 0, pid)
	})

	t.Run("returns false when process is not running (stale PID)", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Use a PID that's unlikely to exist
		err := os.WriteFile(d.PIDFile(), []byte("999999999"), 0644)
		require.NoError(t, err)

		running, pid := d.IsRunning()
		assert.False(t, running)
		assert.Equal(t, 0, pid)

		// PID file should be cleaned up
		_, err = os.Stat(d.PIDFile())
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("returns true when process is running", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Use our own PID which is always running
		myPID := os.Getpid()
		err := os.WriteFile(d.PIDFile(), []byte(fmt.Sprintf("%d", myPID)), 0644)
		require.NoError(t, err)

		running, pid := d.IsRunning()
		assert.True(t, running)
		assert.Equal(t, myPID, pid)
	})
}

func TestDaemon_Stop(t *testing.T) {
	t.Run("AC-02: no error if not running (idempotent)", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		err := d.Stop()
		assert.NoError(t, err)
	})

	t.Run("cleans up stale PID file", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Create stale PID file
		err := os.WriteFile(d.PIDFile(), []byte("999999999"), 0644)
		require.NoError(t, err)

		err = d.Stop()
		assert.NoError(t, err)

		// PID file should be removed
		_, err = os.Stat(d.PIDFile())
		assert.True(t, os.IsNotExist(err))
	})
}

func TestDaemon_GetStatus(t *testing.T) {
	t.Run("AC-02: returns not running status when daemon stopped", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		status, err := d.GetStatus()
		require.NoError(t, err)
		assert.False(t, status.Running)
		assert.Equal(t, 0, status.PID)
	})

	t.Run("AC-01: returns running status with PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Create PID file with our own PID
		myPID := os.Getpid()
		err := os.WriteFile(d.PIDFile(), []byte(fmt.Sprintf("%d", myPID)), 0644)
		require.NoError(t, err)

		status, err := d.GetStatus()
		require.NoError(t, err)
		assert.True(t, status.Running)
		assert.Equal(t, myPID, status.PID)
	})

	t.Run("AC-01: includes status file data when available", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Create PID file with our own PID
		myPID := os.Getpid()
		err := os.WriteFile(d.PIDFile(), []byte(fmt.Sprintf("%d", myPID)), 0644)
		require.NoError(t, err)

		// Create status file
		startTime := time.Now().Add(-1 * time.Hour)
		lastSync := time.Now().Add(-5 * time.Second)
		statusData := &Status{
			Running:        true,
			PID:            myPID,
			StartTime:      startTime,
			LastSync:       lastSync,
			StashesWatched: 2,
		}
		data, err := json.Marshal(statusData)
		require.NoError(t, err)
		err = os.WriteFile(d.StatusFile(), data, 0644)
		require.NoError(t, err)

		status, err := d.GetStatus()
		require.NoError(t, err)
		assert.True(t, status.Running)
		assert.Equal(t, myPID, status.PID)
		assert.Equal(t, 2, status.StashesWatched)
		assert.True(t, status.UptimeSeconds > 0)
	})
}

func TestDaemon_LogExists(t *testing.T) {
	t.Run("returns false when log file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		assert.False(t, d.LogExists())
	})

	t.Run("returns true when log file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		err := os.WriteFile(d.LogFile(), []byte("log entry\n"), 0644)
		require.NoError(t, err)

		assert.True(t, d.LogExists())
	})
}

func TestDaemon_UpdateStatus(t *testing.T) {
	t.Run("creates status file", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		lastSync := time.Now()
		err := d.UpdateStatus(lastSync, 3)
		require.NoError(t, err)

		// Verify file was created
		data, err := os.ReadFile(d.StatusFile())
		require.NoError(t, err)

		var status Status
		err = json.Unmarshal(data, &status)
		require.NoError(t, err)

		assert.Equal(t, 3, status.StashesWatched)
		assert.WithinDuration(t, lastSync, status.LastSync, time.Second)
	})

	t.Run("updates existing status file", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(tmpDir)

		// Create initial status
		err := d.UpdateStatus(time.Now(), 1)
		require.NoError(t, err)

		// Update status
		newSync := time.Now()
		err = d.UpdateStatus(newSync, 5)
		require.NoError(t, err)

		// Verify update
		data, err := os.ReadFile(d.StatusFile())
		require.NoError(t, err)

		var status Status
		err = json.Unmarshal(data, &status)
		require.NoError(t, err)

		assert.Equal(t, 5, status.StashesWatched)
	})
}
