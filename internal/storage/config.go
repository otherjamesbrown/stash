package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/stash/internal/model"
)

// ConfigStore manages stash configuration files.
type ConfigStore struct {
	baseDir string // .stash directory
}

// NewConfigStore creates a new config store.
func NewConfigStore(baseDir string) *ConfigStore {
	return &ConfigStore{baseDir: baseDir}
}

// getConfigPath returns the path to config.json for a stash.
func (s *ConfigStore) getConfigPath(stashName string) string {
	return filepath.Join(s.baseDir, stashName, "config.json")
}

// getStashDir returns the stash directory path.
func (s *ConfigStore) getStashDir(stashName string) string {
	return filepath.Join(s.baseDir, stashName)
}

// WriteConfig writes a stash configuration to config.json.
func (s *ConfigStore) WriteConfig(stash *model.Stash) error {
	dir := s.getStashDir(stash.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create stash directory: %w", err)
	}

	configPath := s.getConfigPath(stash.Name)

	data, err := json.MarshalIndent(stash, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	data = append(data, '\n')

	// Write atomically via temp file
	tmpFile, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ReadConfig reads a stash configuration from config.json.
func (s *ConfigStore) ReadConfig(stashName string) (*model.Stash, error) {
	configPath := s.getConfigPath(stashName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.ErrStashNotFound
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var stash model.Stash
	if err := json.Unmarshal(data, &stash); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &stash, nil
}

// DeleteConfig removes a stash's configuration directory.
func (s *ConfigStore) DeleteConfig(stashName string) error {
	dir := s.getStashDir(stashName)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete stash directory: %w", err)
	}
	return nil
}

// Exists returns true if the stash config exists.
func (s *ConfigStore) Exists(stashName string) bool {
	configPath := s.getConfigPath(stashName)
	_, err := os.Stat(configPath)
	return err == nil
}

// ListStashDirs returns all stash directory names.
func (s *ConfigStore) ListStashDirs() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read stash directory: %w", err)
	}

	var stashes []string
	for _, entry := range entries {
		if entry.IsDir() && !isHiddenOrMeta(entry.Name()) {
			// Check if it has a config.json
			configPath := s.getConfigPath(entry.Name())
			if _, err := os.Stat(configPath); err == nil {
				stashes = append(stashes, entry.Name())
			}
		}
	}

	return stashes, nil
}

// isHiddenOrMeta returns true for hidden or meta directories.
func isHiddenOrMeta(name string) bool {
	return name[0] == '.' || name[0] == '_'
}
