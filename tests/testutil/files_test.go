package testutil

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempDir(t *testing.T) {
	tmpDir := TempDir(t)

	require.NotEmpty(t, tmpDir)
	assert.True(t, DirExists(t, tmpDir))
}

func TestWriteCSV(t *testing.T) {
	tmpDir := TempDir(t)
	content := "Name,Value\nfoo,1\nbar,2"

	path := WriteCSV(t, tmpDir, "test.csv", content)

	assert.True(t, FileExists(t, path))
	assert.Equal(t, filepath.Join(tmpDir, "test.csv"), path)

	readContent := ReadFile(t, path)
	assert.Equal(t, content, readContent)
}

func TestWriteJSON(t *testing.T) {
	tmpDir := TempDir(t)
	content := `{"name": "test", "value": 42}`

	path := WriteJSON(t, tmpDir, "test.json", content)

	assert.True(t, FileExists(t, path))
	assert.Equal(t, filepath.Join(tmpDir, "test.json"), path)

	readContent := ReadFile(t, path)
	assert.Equal(t, content, readContent)
}

func TestWriteFile(t *testing.T) {
	tmpDir := TempDir(t)
	content := "Hello, World!"

	path := WriteFile(t, tmpDir, "hello.txt", content)

	assert.True(t, FileExists(t, path))
	readContent := ReadFile(t, path)
	assert.Equal(t, content, readContent)
}

func TestFileExists(t *testing.T) {
	tmpDir := TempDir(t)

	t.Run("returns false for non-existent file", func(t *testing.T) {
		assert.False(t, FileExists(t, filepath.Join(tmpDir, "nonexistent.txt")))
	})

	t.Run("returns true for existing file", func(t *testing.T) {
		path := WriteFile(t, tmpDir, "exists.txt", "content")
		assert.True(t, FileExists(t, path))
	})

	t.Run("returns false for directory", func(t *testing.T) {
		assert.False(t, FileExists(t, tmpDir))
	})
}

func TestDirExists(t *testing.T) {
	tmpDir := TempDir(t)

	t.Run("returns true for existing directory", func(t *testing.T) {
		assert.True(t, DirExists(t, tmpDir))
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		assert.False(t, DirExists(t, filepath.Join(tmpDir, "nonexistent")))
	})

	t.Run("returns false for file", func(t *testing.T) {
		path := WriteFile(t, tmpDir, "file.txt", "content")
		assert.False(t, DirExists(t, path))
	})
}

func TestMustMkdir(t *testing.T) {
	tmpDir := TempDir(t)
	newDir := filepath.Join(tmpDir, "subdir", "nested")

	MustMkdir(t, newDir)

	assert.True(t, DirExists(t, newDir))
}

func TestMustRemove(t *testing.T) {
	tmpDir := TempDir(t)
	path := WriteFile(t, tmpDir, "toremove.txt", "content")

	assert.True(t, FileExists(t, path))
	MustRemove(t, path)
	assert.False(t, FileExists(t, path))
}

func TestMustRemoveAll(t *testing.T) {
	tmpDir := TempDir(t)
	subDir := filepath.Join(tmpDir, "subdir")
	MustMkdir(t, subDir)
	WriteFile(t, subDir, "file.txt", "content")

	assert.True(t, DirExists(t, subDir))
	MustRemoveAll(t, subDir)
	assert.False(t, DirExists(t, subDir))
}

func TestCopyFile(t *testing.T) {
	tmpDir := TempDir(t)
	content := "original content"
	src := WriteFile(t, tmpDir, "source.txt", content)
	dst := filepath.Join(tmpDir, "destination.txt")

	CopyFile(t, src, dst)

	assert.True(t, FileExists(t, dst))
	assert.Equal(t, content, ReadFile(t, dst))
}
