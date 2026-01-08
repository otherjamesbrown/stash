package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailLog(t *testing.T) {
	t.Run("returns nil for non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "nonexistent.log")

		lines, err := TailLog(logPath, 10)
		require.NoError(t, err)
		assert.Nil(t, lines)
	})

	t.Run("returns nil for empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "empty.log")

		err := os.WriteFile(logPath, []byte(""), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 10)
		require.NoError(t, err)
		assert.Nil(t, lines)
	})

	t.Run("returns all lines when less than requested", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		content := "line 1\nline 2\nline 3\n"
		err := os.WriteFile(logPath, []byte(content), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"line 1", "line 2", "line 3"}, lines)
	})

	t.Run("returns last n lines when more exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
		err := os.WriteFile(logPath, []byte(content), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 3)
		require.NoError(t, err)
		assert.Equal(t, []string{"line 3", "line 4", "line 5"}, lines)
	})

	t.Run("handles file without trailing newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		content := "line 1\nline 2\nline 3"
		err := os.WriteFile(logPath, []byte(content), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"line 1", "line 2", "line 3"}, lines)
	})

	t.Run("handles single line", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		content := "single line\n"
		err := os.WriteFile(logPath, []byte(content), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"single line"}, lines)
	})

	t.Run("handles large file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		// Create a file larger than buffer size
		var builder strings.Builder
		for i := 0; i < 1000; i++ {
			builder.WriteString("this is log line number ")
			builder.WriteString(string(rune('0' + i%10)))
			builder.WriteString(" with some padding text\n")
		}
		err := os.WriteFile(logPath, []byte(builder.String()), 0644)
		require.NoError(t, err)

		lines, err := TailLog(logPath, 5)
		require.NoError(t, err)
		assert.Len(t, lines, 5)
	})
}

func TestSplitLines(t *testing.T) {
	t.Run("splits on newlines", func(t *testing.T) {
		lines := splitLines("a\nb\nc")
		assert.Equal(t, []string{"a", "b", "c"}, lines)
	})

	t.Run("handles trailing newline", func(t *testing.T) {
		lines := splitLines("a\nb\n")
		assert.Equal(t, []string{"a", "b"}, lines)
	})

	t.Run("handles empty string", func(t *testing.T) {
		lines := splitLines("")
		assert.Nil(t, lines)
	})

	t.Run("handles single line no newline", func(t *testing.T) {
		lines := splitLines("single")
		assert.Equal(t, []string{"single"}, lines)
	})

	t.Run("handles consecutive newlines", func(t *testing.T) {
		lines := splitLines("a\n\nb")
		assert.Equal(t, []string{"a", "", "b"}, lines)
	})
}

func TestNewProcess(t *testing.T) {
	t.Run("creates process with correct paths", func(t *testing.T) {
		baseDir := "/tmp/test-stash/.stash"
		proc := NewProcess(baseDir)

		assert.NotNil(t, proc)
		assert.NotNil(t, proc.daemon)
		assert.Equal(t, baseDir, proc.daemon.BaseDir())
		// stashesDir should be parent of baseDir
		assert.Equal(t, "/tmp/test-stash", proc.stashesDir)
	})
}
