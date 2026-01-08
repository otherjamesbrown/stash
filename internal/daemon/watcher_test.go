package daemon

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileWatcher_WatchRecordsJSONL tests that the watcher detects changes to records.jsonl files.
// Maps to bead st-ta2: "Watch .stash/*/records.jsonl"
func TestFileWatcher_WatchRecordsJSONL(t *testing.T) {
	t.Run("detects write to records.jsonl", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial records.jsonl file
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var rebuildCalled atomic.Bool
		rebuildFn := func(stashName string) error {
			rebuildCalled.Store(true)
			assert.Equal(t, "test-stash", stashName)
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Write to records.jsonl
		err = os.WriteFile(jsonlPath, []byte(`{"id":"rec-001"}`+"\n"), 0644)
		require.NoError(t, err)

		// Wait for debounce + processing
		time.Sleep(200 * time.Millisecond)

		assert.True(t, rebuildCalled.Load(), "rebuild should have been called for records.jsonl change")
	})
}

// TestFileWatcher_WatchConfigJSON tests that the watcher detects changes to config.json files.
// Maps to bead st-ta2: "Watch .stash/*/config.json"
func TestFileWatcher_WatchConfigJSON(t *testing.T) {
	t.Run("detects write to config.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial config.json file
		configPath := filepath.Join(stashDir, "config.json")
		require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0644))

		var rebuildCalled atomic.Bool
		rebuildFn := func(stashName string) error {
			rebuildCalled.Store(true)
			assert.Equal(t, "test-stash", stashName)
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Write to config.json
		err = os.WriteFile(configPath, []byte(`{"name":"test-stash"}`), 0644)
		require.NoError(t, err)

		// Wait for debounce + processing
		time.Sleep(200 * time.Millisecond)

		assert.True(t, rebuildCalled.Load(), "rebuild should have been called for config.json change")
	})
}

// TestFileWatcher_Debounce tests that rapid changes are debounced.
// Maps to bead st-ta2: "Debounce rapid changes (100ms)"
func TestFileWatcher_Debounce(t *testing.T) {
	t.Run("debounces rapid file changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial records.jsonl file
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var rebuildCount atomic.Int32
		rebuildFn := func(stashName string) error {
			rebuildCount.Add(1)
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Make 5 rapid changes (faster than debounce interval)
		for i := 0; i < 5; i++ {
			err = os.WriteFile(jsonlPath, []byte(`{"id":"rec-`+string(rune('0'+i))+`"}`+"\n"), 0644)
			require.NoError(t, err)
			time.Sleep(20 * time.Millisecond) // 20ms between writes, less than 100ms debounce
		}

		// Wait for final debounce to fire
		time.Sleep(200 * time.Millisecond)

		// Should only have been called once or twice (debounced), not 5 times
		count := rebuildCount.Load()
		assert.LessOrEqual(t, count, int32(2), "rapid changes should be debounced, got %d calls", count)
		assert.GreaterOrEqual(t, count, int32(1), "at least one rebuild should occur")
	})
}

// TestFileWatcher_MultipleStashes tests watching multiple stash directories.
// Maps to bead st-ta2: "Watch .stash/*/records.jsonl"
func TestFileWatcher_MultipleStashes(t *testing.T) {
	t.Run("watches multiple stash directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stash1Dir := filepath.Join(baseDir, "stash1")
		stash2Dir := filepath.Join(baseDir, "stash2")
		require.NoError(t, os.MkdirAll(stash1Dir, 0755))
		require.NoError(t, os.MkdirAll(stash2Dir, 0755))

		// Create initial files
		jsonl1 := filepath.Join(stash1Dir, "records.jsonl")
		jsonl2 := filepath.Join(stash2Dir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonl1, []byte(""), 0644))
		require.NoError(t, os.WriteFile(jsonl2, []byte(""), 0644))

		var mu sync.Mutex
		stashesRebuilt := make(map[string]bool)
		rebuildFn := func(stashName string) error {
			mu.Lock()
			stashesRebuilt[stashName] = true
			mu.Unlock()
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Modify both stashes
		require.NoError(t, os.WriteFile(jsonl1, []byte(`{"id":"rec-1"}`+"\n"), 0644))
		time.Sleep(150 * time.Millisecond) // Wait for debounce
		require.NoError(t, os.WriteFile(jsonl2, []byte(`{"id":"rec-2"}`+"\n"), 0644))
		time.Sleep(200 * time.Millisecond) // Wait for debounce

		mu.Lock()
		defer mu.Unlock()
		assert.True(t, stashesRebuilt["stash1"], "stash1 should have been rebuilt")
		assert.True(t, stashesRebuilt["stash2"], "stash2 should have been rebuilt")
	})
}

// TestFileWatcher_LogEvents tests that watch events are logged.
// Maps to bead st-ta2: Implicit requirement for observability
func TestFileWatcher_LogEvents(t *testing.T) {
	t.Run("logs watch events via callback", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial file
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var loggedMessages []string
		var logMu sync.Mutex
		logFn := func(msg string, args ...interface{}) {
			logMu.Lock()
			loggedMessages = append(loggedMessages, msg)
			logMu.Unlock()
		}

		rebuildFn := func(stashName string) error {
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, logFn)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Trigger a file change
		require.NoError(t, os.WriteFile(jsonlPath, []byte(`{"id":"rec-1"}`+"\n"), 0644))

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		logMu.Lock()
		defer logMu.Unlock()
		assert.NotEmpty(t, loggedMessages, "should have logged at least one message")
	})
}

// TestFileWatcher_IgnoresNonWatchedFiles tests that the watcher ignores irrelevant files.
func TestFileWatcher_IgnoresNonWatchedFiles(t *testing.T) {
	t.Run("ignores non-watched files", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial records.jsonl so the watcher has something valid
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var rebuildCalled atomic.Bool
		rebuildFn := func(stashName string) error {
			rebuildCalled.Store(true)
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Write to a non-watched file
		otherPath := filepath.Join(stashDir, "other.txt")
		require.NoError(t, os.WriteFile(otherPath, []byte("some content"), 0644))

		// Wait for potential debounce
		time.Sleep(200 * time.Millisecond)

		assert.False(t, rebuildCalled.Load(), "rebuild should not be called for non-watched file")
	})
}

// TestFileWatcher_HandleErrors tests graceful error handling.
func TestFileWatcher_HandleErrors(t *testing.T) {
	t.Run("continues on rebuild error", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial file
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var rebuildCount atomic.Int32
		rebuildFn := func(stashName string) error {
			rebuildCount.Add(1)
			return assert.AnError // Return an error
		}

		var errorLogged atomic.Bool
		logFn := func(msg string, args ...interface{}) {
			if msg == "Error rebuilding cache for stash %s: %v" || msg == "rebuild error" {
				errorLogged.Store(true)
			}
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, logFn)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Trigger changes
		require.NoError(t, os.WriteFile(jsonlPath, []byte(`{"id":"rec-1"}`+"\n"), 0644))
		time.Sleep(200 * time.Millisecond)

		// Watcher should still be running despite error
		require.NoError(t, os.WriteFile(jsonlPath, []byte(`{"id":"rec-2"}`+"\n"), 0644))
		time.Sleep(200 * time.Millisecond)

		// Should have been called at least twice
		assert.GreaterOrEqual(t, rebuildCount.Load(), int32(2), "watcher should continue after errors")
	})

	t.Run("handles non-existent base directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, "nonexistent", ".stash")

		rebuildFn := func(stashName string) error { return nil }
		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		// Should not error - just won't watch anything
		err = watcher.Start()
		require.NoError(t, err)
	})
}

// TestFileWatcher_Close tests proper cleanup on close.
func TestFileWatcher_Close(t *testing.T) {
	t.Run("stops watching on close", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		stashDir := filepath.Join(baseDir, "test-stash")
		require.NoError(t, os.MkdirAll(stashDir, 0755))

		// Create initial file
		jsonlPath := filepath.Join(stashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(jsonlPath, []byte(""), 0644))

		var rebuildCount atomic.Int32
		rebuildFn := func(stashName string) error {
			rebuildCount.Add(1)
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for initialization
		time.Sleep(50 * time.Millisecond)

		// Close the watcher
		watcher.Close()

		// Record count before
		countBefore := rebuildCount.Load()

		// Try to trigger a change after close
		require.NoError(t, os.WriteFile(jsonlPath, []byte(`{"id":"rec-1"}`+"\n"), 0644))
		time.Sleep(200 * time.Millisecond)

		// Should not have received any more events
		assert.Equal(t, countBefore, rebuildCount.Load(), "should not process events after close")
	})
}

// TestFileWatcher_NewStashDirectory tests that new stash directories are automatically watched.
func TestFileWatcher_NewStashDirectory(t *testing.T) {
	t.Run("watches new stash directories created after start", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, ".stash")
		require.NoError(t, os.MkdirAll(baseDir, 0755))

		var mu sync.Mutex
		stashesRebuilt := make(map[string]bool)
		rebuildFn := func(stashName string) error {
			mu.Lock()
			stashesRebuilt[stashName] = true
			mu.Unlock()
			return nil
		}

		watcher, err := NewWatcher(baseDir, rebuildFn, nil)
		require.NoError(t, err)
		defer watcher.Close()

		err = watcher.Start()
		require.NoError(t, err)

		// Wait for watcher to initialize
		time.Sleep(50 * time.Millisecond)

		// Create a new stash directory
		newStashDir := filepath.Join(baseDir, "new-stash")
		require.NoError(t, os.MkdirAll(newStashDir, 0755))

		// Wait for directory watch to be added
		time.Sleep(100 * time.Millisecond)

		// Create and modify records.jsonl in the new stash
		newJsonl := filepath.Join(newStashDir, "records.jsonl")
		require.NoError(t, os.WriteFile(newJsonl, []byte(`{"id":"rec-1"}`+"\n"), 0644))

		// Wait for debounce
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		assert.True(t, stashesRebuilt["new-stash"], "new stash directory should be watched")
	})
}
