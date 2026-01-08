package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteCSV creates a CSV file for import testing.
// Returns the full path to the created file.
func WriteCSV(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write CSV file %s: %v", path, err)
	}
	return path
}

// WriteJSON creates a JSON file for import testing.
// Returns the full path to the created file.
func WriteJSON(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write JSON file %s: %v", path, err)
	}
	return path
}

// WriteFile creates a file with arbitrary content.
// Returns the full path to the created file.
func WriteFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
	return path
}

// FileExists checks if a file exists.
func FileExists(t *testing.T, path string) bool {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("error checking file %s: %v", path, err)
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(t *testing.T, path string) bool {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("error checking directory %s: %v", path, err)
	}
	return info.IsDir()
}

// ReadFile reads the content of a file and returns it as a string.
func ReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(data)
}

// MustMkdir creates a directory and all parent directories.
// Fails the test if the directory cannot be created.
func MustMkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

// MustRemove removes a file or empty directory.
// Fails the test if the removal fails (except if the file doesn't exist).
func MustRemove(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove %s: %v", path, err)
	}
}

// MustRemoveAll removes a file or directory recursively.
// Fails the test if the removal fails (except if the path doesn't exist).
func MustRemoveAll(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove %s: %v", path, err)
	}
}

// CopyFile copies a file from src to dst.
func CopyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read source file %s: %v", src, err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("failed to write destination file %s: %v", dst, err)
	}
}
