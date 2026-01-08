package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// DefaultDebounceInterval is the default interval to wait after the last change before triggering a rebuild.
	DefaultDebounceInterval = 100 * time.Millisecond
)

// RebuildFunc is called when a stash needs to be rebuilt.
type RebuildFunc func(stashName string) error

// LogFunc is called to log messages.
type LogFunc func(format string, args ...interface{})

// Watcher monitors .stash directories for changes and triggers cache rebuilds.
type Watcher struct {
	baseDir          string
	rebuildFn        RebuildFunc
	logFn            LogFunc
	debounceInterval time.Duration

	watcher   *fsnotify.Watcher
	stopChan  chan struct{}
	doneChan  chan struct{}
	closeOnce sync.Once

	// debounce state per stash
	mu             sync.Mutex
	pendingRebuilds map[string]*time.Timer
}

// NewWatcher creates a new file watcher for the given base directory.
// baseDir is typically the .stash directory.
// rebuildFn is called when a stash needs cache rebuilding.
// logFn is called for logging (can be nil for no logging).
func NewWatcher(baseDir string, rebuildFn RebuildFunc, logFn LogFunc) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if logFn == nil {
		logFn = func(format string, args ...interface{}) {} // no-op
	}

	return &Watcher{
		baseDir:          baseDir,
		rebuildFn:        rebuildFn,
		logFn:            logFn,
		debounceInterval: DefaultDebounceInterval,
		watcher:          fsWatcher,
		stopChan:         make(chan struct{}),
		doneChan:         make(chan struct{}),
		pendingRebuilds:  make(map[string]*time.Timer),
	}, nil
}

// Start begins watching for file changes.
func (w *Watcher) Start() error {
	// Add watch on base directory for new stash directories
	if err := w.addWatchIfExists(w.baseDir); err != nil {
		w.logFn("Warning: could not watch base directory %s: %v", w.baseDir, err)
	}

	// Watch existing stash subdirectories
	if err := w.watchExistingStashes(); err != nil {
		w.logFn("Warning: could not watch existing stashes: %v", err)
	}

	// Start event processing goroutine
	go w.processEvents()

	return nil
}

// Close stops the watcher and cleans up resources.
func (w *Watcher) Close() {
	w.closeOnce.Do(func() {
		close(w.stopChan)
		w.watcher.Close()

		// Cancel any pending debounce timers
		w.mu.Lock()
		for _, timer := range w.pendingRebuilds {
			timer.Stop()
		}
		w.pendingRebuilds = nil
		w.mu.Unlock()

		// Wait for event processing to finish
		<-w.doneChan
	})
}

// watchExistingStashes adds watches for all existing stash directories.
func (w *Watcher) watchExistingStashes() error {
	entries, err := os.ReadDir(w.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Base directory doesn't exist yet
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			stashDir := filepath.Join(w.baseDir, entry.Name())
			if err := w.watcher.Add(stashDir); err != nil {
				w.logFn("Warning: could not watch stash directory %s: %v", stashDir, err)
			} else {
				w.logFn("Watching stash directory: %s", stashDir)
			}
		}
	}

	return nil
}

// addWatchIfExists adds a watch for a directory if it exists.
func (w *Watcher) addWatchIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Directory doesn't exist, no error
	}
	return w.watcher.Add(path)
}

// processEvents handles filesystem events.
func (w *Watcher) processEvents() {
	defer close(w.doneChan)

	for {
		select {
		case <-w.stopChan:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logFn("Watch error: %v", err)
		}
	}
}

// handleEvent processes a single filesystem event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name
	filename := filepath.Base(path)
	parentDir := filepath.Dir(path)

	// Check if this is a new directory being created in the base directory
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() && parentDir == w.baseDir {
			// New stash directory created, add a watch
			if err := w.watcher.Add(path); err != nil {
				w.logFn("Warning: could not watch new stash directory %s: %v", path, err)
			} else {
				w.logFn("Watching new stash directory: %s", path)
			}
			return
		}
	}

	// Check if this is a watched file (records.jsonl or config.json)
	if !w.isWatchedFile(filename) {
		return
	}

	// Only process write events
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	// Extract stash name from path
	stashName := w.extractStashName(path)
	if stashName == "" {
		return
	}

	w.logFn("File change detected: %s (stash: %s)", filename, stashName)

	// Debounce the rebuild
	w.scheduleRebuild(stashName)
}

// isWatchedFile returns true if the filename is one we should watch.
func (w *Watcher) isWatchedFile(filename string) bool {
	return filename == "records.jsonl" || filename == "config.json"
}

// extractStashName extracts the stash name from a file path.
// Returns empty string if the path is not a valid stash file.
func (w *Watcher) extractStashName(path string) string {
	// Path should be: baseDir/stashName/filename
	relPath, err := filepath.Rel(w.baseDir, path)
	if err != nil {
		return ""
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}

	return parts[0]
}

// scheduleRebuild schedules a debounced rebuild for the given stash.
func (w *Watcher) scheduleRebuild(stashName string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel any existing timer
	if timer, exists := w.pendingRebuilds[stashName]; exists {
		timer.Stop()
	}

	// Schedule a new rebuild after debounce interval
	w.pendingRebuilds[stashName] = time.AfterFunc(w.debounceInterval, func() {
		w.doRebuild(stashName)
	})
}

// doRebuild performs the actual rebuild.
func (w *Watcher) doRebuild(stashName string) {
	w.mu.Lock()
	delete(w.pendingRebuilds, stashName)
	w.mu.Unlock()

	w.logFn("Rebuilding cache for stash: %s", stashName)

	if err := w.rebuildFn(stashName); err != nil {
		w.logFn("Error rebuilding cache for stash %s: %v", stashName, err)
	} else {
		w.logFn("Cache rebuild complete for stash: %s", stashName)
	}
}

// StashCount returns the number of stash directories being watched.
func (w *Watcher) StashCount() int {
	entries, err := os.ReadDir(w.baseDir)
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
