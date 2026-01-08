package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/stash/internal/model"
)

func TestStore_CreateAndGetStash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	t.Run("create stash", func(t *testing.T) {
		err := store.CreateStash("test-stash", "ts-", stash)
		require.NoError(t, err)
	})

	t.Run("get stash", func(t *testing.T) {
		retrieved, err := store.GetStash("test-stash")
		require.NoError(t, err)

		assert.Equal(t, "test-stash", retrieved.Name)
		assert.Equal(t, "ts-", retrieved.Prefix)
		assert.Len(t, retrieved.Columns, 1)
	})

	t.Run("create duplicate stash fails", func(t *testing.T) {
		err := store.CreateStash("test-stash", "ts-", stash)
		assert.ErrorIs(t, err, model.ErrStashExists)
	})

	t.Run("get non-existent stash", func(t *testing.T) {
		_, err := store.GetStash("nonexistent")
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})
}

func TestStore_DropStash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	t.Run("drop existing stash", func(t *testing.T) {
		err := store.DropStash("test-stash")
		require.NoError(t, err)

		_, err = store.GetStash("test-stash")
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})

	t.Run("drop non-existent stash fails", func(t *testing.T) {
		err := store.DropStash("nonexistent")
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})
}

func TestStore_ListStashes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stashes := []*model.Stash{
		{Name: "alpha", Prefix: "al-", Created: time.Now(), CreatedBy: "user"},
		{Name: "beta", Prefix: "be-", Created: time.Now(), CreatedBy: "user"},
	}

	for _, s := range stashes {
		err := store.CreateStash(s.Name, s.Prefix, s)
		require.NoError(t, err)
	}

	t.Run("list all stashes", func(t *testing.T) {
		result, err := store.ListStashes()
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestStore_AddColumn(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns:   model.ColumnList{},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	t.Run("add column", func(t *testing.T) {
		col := model.Column{
			Name:    "description",
			Desc:    "Item description",
			Added:   time.Now(),
			AddedBy: "user",
		}

		err := store.AddColumn("test-stash", col)
		require.NoError(t, err)

		// Verify column was added
		retrieved, err := store.GetStash("test-stash")
		require.NoError(t, err)
		assert.Len(t, retrieved.Columns, 1)
		assert.Equal(t, "description", retrieved.Columns[0].Name)
	})

	t.Run("add duplicate column fails", func(t *testing.T) {
		col := model.Column{
			Name:    "description",
			Added:   time.Now(),
			AddedBy: "user",
		}

		err := store.AddColumn("test-stash", col)
		assert.ErrorIs(t, err, model.ErrColumnExists)
	})

	t.Run("add column to non-existent stash fails", func(t *testing.T) {
		col := model.Column{
			Name:    "foo",
			Added:   time.Now(),
			AddedBy: "user",
		}

		err := store.AddColumn("nonexistent", col)
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})
}

func TestStore_RecordCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "user"},
			{Name: "count", Added: time.Now(), AddedBy: "user"},
		},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	now := time.Now()

	record := &model.Record{
		ID:        "ts-abc1",
		CreatedAt: now,
		CreatedBy: "creator",
		UpdatedAt: now,
		UpdatedBy: "creator",
		Fields: map[string]interface{}{
			"name":  "Test Item",
			"count": float64(10),
		},
	}

	t.Run("create record", func(t *testing.T) {
		err := store.CreateRecord("test-stash", record)
		require.NoError(t, err)

		// Verify hash was calculated
		assert.NotEmpty(t, record.Hash)
		assert.Equal(t, model.OpCreate, record.Operation)
	})

	t.Run("get record", func(t *testing.T) {
		retrieved, err := store.GetRecord("test-stash", "ts-abc1")
		require.NoError(t, err)

		assert.Equal(t, "ts-abc1", retrieved.ID)
		assert.Equal(t, "Test Item", retrieved.Fields["name"])
		assert.Equal(t, float64(10), retrieved.Fields["count"])
	})

	t.Run("update record", func(t *testing.T) {
		record.UpdatedAt = time.Now()
		record.UpdatedBy = "updater"
		record.Fields["name"] = "Updated Item"
		record.Fields["count"] = float64(20)

		err := store.UpdateRecord("test-stash", record)
		require.NoError(t, err)

		retrieved, err := store.GetRecord("test-stash", "ts-abc1")
		require.NoError(t, err)

		assert.Equal(t, "Updated Item", retrieved.Fields["name"])
		assert.Equal(t, float64(20), retrieved.Fields["count"])
		assert.Equal(t, "updater", retrieved.UpdatedBy)
	})

	t.Run("delete record", func(t *testing.T) {
		err := store.DeleteRecord("test-stash", "ts-abc1", "deleter")
		require.NoError(t, err)

		// GetRecord should return deleted error
		_, err = store.GetRecord("test-stash", "ts-abc1")
		assert.ErrorIs(t, err, model.ErrRecordDeleted)
	})

	t.Run("delete already deleted fails", func(t *testing.T) {
		err := store.DeleteRecord("test-stash", "ts-abc1", "deleter")
		assert.ErrorIs(t, err, model.ErrRecordDeleted)
	})

	t.Run("restore record", func(t *testing.T) {
		err := store.RestoreRecord("test-stash", "ts-abc1", "restorer")
		require.NoError(t, err)

		retrieved, err := store.GetRecord("test-stash", "ts-abc1")
		require.NoError(t, err)

		assert.False(t, retrieved.IsDeleted())
		assert.Equal(t, "restorer", retrieved.UpdatedBy)
	})

	t.Run("get non-existent record", func(t *testing.T) {
		_, err := store.GetRecord("test-stash", "ts-nonexistent")
		assert.ErrorIs(t, err, model.ErrRecordNotFound)
	})
}

func TestStore_ListRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "user"},
		},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	now := time.Now()

	records := []*model.Record{
		{
			ID:        "ts-abc1",
			CreatedAt: now,
			CreatedBy: "user",
			UpdatedAt: now,
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "First"},
		},
		{
			ID:        "ts-abc2",
			CreatedAt: now.Add(time.Second),
			CreatedBy: "user",
			UpdatedAt: now.Add(time.Second),
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "Second"},
		},
		{
			ID:        "ts-abc1.1",
			ParentID:  "ts-abc1",
			CreatedAt: now.Add(2 * time.Second),
			CreatedBy: "user",
			UpdatedAt: now.Add(2 * time.Second),
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "Child of First"},
		},
	}

	for _, r := range records {
		err := store.CreateRecord("test-stash", r)
		require.NoError(t, err)
	}

	t.Run("list root records", func(t *testing.T) {
		result, err := store.ListRecords("test-stash", ListOptions{})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("list all records", func(t *testing.T) {
		result, err := store.ListRecords("test-stash", ListOptions{ParentID: "*"})
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("list children", func(t *testing.T) {
		result, err := store.GetChildren("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1.1", result[0].ID)
	})
}

func TestStore_GetNextChildSeq(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns:   model.ColumnList{},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	now := time.Now()

	parent := &model.Record{
		ID:        "ts-abc1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{},
	}
	err = store.CreateRecord("test-stash", parent)
	require.NoError(t, err)

	t.Run("first child seq", func(t *testing.T) {
		seq, err := store.GetNextChildSeq("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Equal(t, 1, seq)
	})

	// Add a child
	child1 := &model.Record{
		ID:        "ts-abc1.1",
		ParentID:  "ts-abc1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{},
	}
	err = store.CreateRecord("test-stash", child1)
	require.NoError(t, err)

	t.Run("next child seq after adding child", func(t *testing.T) {
		seq, err := store.GetNextChildSeq("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Equal(t, 2, seq)
	})
}

func TestStore_RebuildCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "user"},
		},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	now := time.Now()

	// Create some records
	record := &model.Record{
		ID:        "ts-abc1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{"name": "Test"},
	}
	err = store.CreateRecord("test-stash", record)
	require.NoError(t, err)

	// Update the record
	record.Fields["name"] = "Updated"
	record.UpdatedAt = now.Add(time.Second)
	err = store.UpdateRecord("test-stash", record)
	require.NoError(t, err)

	// Clear SQLite and rebuild
	err = store.sqlite.ClearTable("test-stash")
	require.NoError(t, err)

	// Verify cache is empty
	records, err := store.ListRecords("test-stash", ListOptions{ParentID: "*"})
	require.NoError(t, err)
	assert.Empty(t, records)

	// Rebuild cache
	err = store.RebuildCache("test-stash")
	require.NoError(t, err)

	// Verify record was restored
	records, err = store.ListRecords("test-stash", ListOptions{ParentID: "*"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Updated", records[0].Fields["name"])
}

func TestStore_FlushToJSONL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "user"},
		},
	}

	err = store.CreateStash("test-stash", "ts-", stash)
	require.NoError(t, err)

	now := time.Now()

	// Create a record
	record := &model.Record{
		ID:        "ts-abc1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{"name": "Test"},
	}
	err = store.CreateRecord("test-stash", record)
	require.NoError(t, err)

	// Update the record multiple times
	for i := 0; i < 5; i++ {
		record.UpdatedAt = now.Add(time.Duration(i+1) * time.Second)
		record.Fields["name"] = "Updated"
		err = store.UpdateRecord("test-stash", record)
		require.NoError(t, err)
	}

	// JSONL should have 6 entries (1 create + 5 updates)
	jsonlRecords, err := store.jsonl.ReadAllRecords("test-stash")
	require.NoError(t, err)
	assert.Len(t, jsonlRecords, 6)

	// Flush to compact
	err = store.FlushToJSONL("test-stash")
	require.NoError(t, err)

	// JSONL should now have 1 entry
	jsonlRecords, err = store.jsonl.ReadAllRecords("test-stash")
	require.NoError(t, err)
	assert.Len(t, jsonlRecords, 1)
	assert.Equal(t, "Updated", jsonlRecords[0].Fields["name"])
}
