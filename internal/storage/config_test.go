package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/stash/internal/model"
)

func TestConfigStore_WriteAndReadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewConfigStore(tmpDir)

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now().Truncate(time.Second),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Desc: "Item name", Added: time.Now().Truncate(time.Second), AddedBy: "test-user"},
			{Name: "value", Added: time.Now().Truncate(time.Second), AddedBy: "test-user"},
		},
	}

	t.Run("write config", func(t *testing.T) {
		err := store.WriteConfig(stash)
		require.NoError(t, err)

		// Verify file exists
		configPath := filepath.Join(tmpDir, "test-stash", "config.json")
		assert.FileExists(t, configPath)
	})

	t.Run("read config", func(t *testing.T) {
		retrieved, err := store.ReadConfig("test-stash")
		require.NoError(t, err)

		assert.Equal(t, stash.Name, retrieved.Name)
		assert.Equal(t, stash.Prefix, retrieved.Prefix)
		assert.Equal(t, stash.CreatedBy, retrieved.CreatedBy)
		assert.Len(t, retrieved.Columns, 2)
		assert.Equal(t, "name", retrieved.Columns[0].Name)
		assert.Equal(t, "Item name", retrieved.Columns[0].Desc)
	})

	t.Run("read non-existent config", func(t *testing.T) {
		_, err := store.ReadConfig("nonexistent")
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})
}

func TestConfigStore_Exists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewConfigStore(tmpDir)

	t.Run("does not exist initially", func(t *testing.T) {
		assert.False(t, store.Exists("test-stash"))
	})

	t.Run("exists after write", func(t *testing.T) {
		stash := &model.Stash{
			Name:      "test-stash",
			Prefix:    "ts-",
			Created:   time.Now(),
			CreatedBy: "user",
		}
		err := store.WriteConfig(stash)
		require.NoError(t, err)

		assert.True(t, store.Exists("test-stash"))
	})
}

func TestConfigStore_DeleteConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewConfigStore(tmpDir)

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
	}
	err = store.WriteConfig(stash)
	require.NoError(t, err)
	require.True(t, store.Exists("test-stash"))

	t.Run("delete existing config", func(t *testing.T) {
		err := store.DeleteConfig("test-stash")
		require.NoError(t, err)

		assert.False(t, store.Exists("test-stash"))

		// Directory should be removed
		stashDir := filepath.Join(tmpDir, "test-stash")
		_, err = os.Stat(stashDir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("delete non-existent succeeds", func(t *testing.T) {
		err := store.DeleteConfig("nonexistent")
		require.NoError(t, err)
	})
}

func TestConfigStore_ListStashDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewConfigStore(tmpDir)

	// Create several stashes
	stashes := []string{"alpha", "beta", "gamma"}
	for _, name := range stashes {
		stash := &model.Stash{
			Name:      name,
			Prefix:    name[:2] + "-",
			Created:   time.Now(),
			CreatedBy: "user",
		}
		err := store.WriteConfig(stash)
		require.NoError(t, err)
	}

	// Create a directory without config.json (should be ignored)
	err = os.MkdirAll(filepath.Join(tmpDir, "invalid-dir"), 0755)
	require.NoError(t, err)

	// Create hidden directory (should be ignored)
	err = os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755)
	require.NoError(t, err)

	t.Run("list stash directories", func(t *testing.T) {
		dirs, err := store.ListStashDirs()
		require.NoError(t, err)
		assert.Len(t, dirs, 3)
		assert.Contains(t, dirs, "alpha")
		assert.Contains(t, dirs, "beta")
		assert.Contains(t, dirs, "gamma")
	})
}

func TestConfigStore_UpdateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewConfigStore(tmpDir)

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns:   model.ColumnList{},
	}
	err = store.WriteConfig(stash)
	require.NoError(t, err)

	t.Run("update config adds column", func(t *testing.T) {
		stash.Columns = append(stash.Columns, model.Column{
			Name:    "description",
			Added:   time.Now(),
			AddedBy: "user",
		})

		err := store.WriteConfig(stash)
		require.NoError(t, err)

		retrieved, err := store.ReadConfig("test-stash")
		require.NoError(t, err)
		assert.Len(t, retrieved.Columns, 1)
		assert.Equal(t, "description", retrieved.Columns[0].Name)
	})
}

func TestConfigStore_EmptyBaseDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Use a non-existent subdirectory
	baseDir := filepath.Join(tmpDir, "nonexistent", "stash")
	store := NewConfigStore(baseDir)

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
	}

	t.Run("write creates directories", func(t *testing.T) {
		err := store.WriteConfig(stash)
		require.NoError(t, err)

		assert.True(t, store.Exists("test-stash"))
	})
}
