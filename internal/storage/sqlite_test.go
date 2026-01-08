package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/stash/internal/model"
)

func TestSQLiteCache_CreateStashTable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
			{Name: "value", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	t.Run("create table with columns", func(t *testing.T) {
		err := cache.CreateStashTable(stash)
		require.NoError(t, err)

		// Verify table exists
		exists, err := cache.TableExists("test-stash")
		require.NoError(t, err)
		assert.True(t, exists)

		// Verify columns exist
		colExists, err := cache.columnExists("test_stash", "name")
		require.NoError(t, err)
		assert.True(t, colExists)

		colExists, err = cache.columnExists("test_stash", "value")
		require.NoError(t, err)
		assert.True(t, colExists)
	})

	t.Run("retrieve stash config", func(t *testing.T) {
		retrieved, err := cache.GetStash("test-stash")
		require.NoError(t, err)

		assert.Equal(t, "test-stash", retrieved.Name)
		assert.Equal(t, "ts-", retrieved.Prefix)
		assert.Len(t, retrieved.Columns, 2)
	})
}

func TestSQLiteCache_AddColumn(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	t.Run("add new column", func(t *testing.T) {
		err := cache.AddColumn("test-stash", "description")
		require.NoError(t, err)

		exists, err := cache.columnExists("test_stash", "description")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("add column idempotent", func(t *testing.T) {
		err := cache.AddColumn("test-stash", "description")
		require.NoError(t, err) // Should not error
	})
}

func TestSQLiteCache_UpsertAndGetRecord(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
			{Name: "count", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"name", "count"}
	now := time.Now().Truncate(time.Second)

	record := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash123456",
		CreatedAt: now,
		CreatedBy: "creator",
		UpdatedAt: now,
		UpdatedBy: "creator",
		Fields: map[string]interface{}{
			"name":  "Test Item",
			"count": float64(42),
		},
	}

	t.Run("insert record", func(t *testing.T) {
		err := cache.UpsertRecord("test-stash", record, columns)
		require.NoError(t, err)
	})

	t.Run("get record", func(t *testing.T) {
		retrieved, err := cache.GetRecord("test-stash", "ts-abc1", columns)
		require.NoError(t, err)

		assert.Equal(t, "ts-abc1", retrieved.ID)
		assert.Equal(t, "hash123456", retrieved.Hash)
		assert.Equal(t, "creator", retrieved.CreatedBy)
		assert.Equal(t, "Test Item", retrieved.Fields["name"])
		assert.Equal(t, float64(42), retrieved.Fields["count"])
	})

	t.Run("update record", func(t *testing.T) {
		updateTime := time.Now().Truncate(time.Second)
		record.UpdatedAt = updateTime
		record.UpdatedBy = "updater"
		record.Fields["name"] = "Updated Item"
		record.Fields["count"] = float64(100)
		record.Hash = "hash234567"

		err := cache.UpsertRecord("test-stash", record, columns)
		require.NoError(t, err)

		retrieved, err := cache.GetRecord("test-stash", "ts-abc1", columns)
		require.NoError(t, err)

		assert.Equal(t, "hash234567", retrieved.Hash)
		assert.Equal(t, "updater", retrieved.UpdatedBy)
		assert.Equal(t, "Updated Item", retrieved.Fields["name"])
		assert.Equal(t, float64(100), retrieved.Fields["count"])
	})

	t.Run("get non-existent record", func(t *testing.T) {
		_, err := cache.GetRecord("test-stash", "ts-nonexistent", columns)
		assert.ErrorIs(t, err, model.ErrRecordNotFound)
	})
}

func TestSQLiteCache_ListRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"name"}
	now := time.Now().Truncate(time.Second)

	// Insert test records
	records := []*model.Record{
		{
			ID:        "ts-abc1",
			Hash:      "hash1",
			CreatedAt: now,
			CreatedBy: "user",
			UpdatedAt: now,
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "First"},
		},
		{
			ID:        "ts-abc2",
			Hash:      "hash2",
			CreatedAt: now.Add(time.Second),
			CreatedBy: "user",
			UpdatedAt: now.Add(time.Second),
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "Second"},
		},
		{
			ID:        "ts-abc1.1",
			Hash:      "hash3",
			ParentID:  "ts-abc1",
			CreatedAt: now.Add(2 * time.Second),
			CreatedBy: "user",
			UpdatedAt: now.Add(2 * time.Second),
			UpdatedBy: "user",
			Fields:    map[string]interface{}{"name": "Child"},
		},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("list all root records", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "", // Root records only
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("list all records including children", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*", // All records
		})
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("list children of parent", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "ts-abc1",
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1.1", result[0].ID)
	})

	t.Run("list with limit", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Limit:    2,
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("list with offset", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Offset:   1,
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("list descending", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID:   "*",
			Descending: true,
		})
		require.NoError(t, err)
		require.Len(t, result, 3)
		// Most recently updated should be first
		assert.Equal(t, "ts-abc1.1", result[0].ID)
	})
}

func TestSQLiteCache_DeletedRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{}
	now := time.Now().Truncate(time.Second)
	deletedAt := now.Add(time.Hour)

	activeRecord := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{},
	}

	deletedRecord := &model.Record{
		ID:        "ts-abc2",
		Hash:      "hash2",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: deletedAt,
		UpdatedBy: "deleter",
		DeletedAt: &deletedAt,
		DeletedBy: "deleter",
		Fields:    map[string]interface{}{},
	}

	err = cache.UpsertRecord("test-stash", activeRecord, columns)
	require.NoError(t, err)

	err = cache.UpsertRecord("test-stash", deletedRecord, columns)
	require.NoError(t, err)

	t.Run("list excludes deleted by default", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1", result[0].ID)
	})

	t.Run("list includes deleted when requested", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID:       "*",
			IncludeDeleted: true,
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("get deleted record", func(t *testing.T) {
		retrieved, err := cache.GetRecord("test-stash", "ts-abc2", columns)
		require.NoError(t, err)
		assert.True(t, retrieved.IsDeleted())
		assert.Equal(t, "deleter", retrieved.DeletedBy)
	})
}

func TestSQLiteCache_DropStashTable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	t.Run("drop existing table", func(t *testing.T) {
		err := cache.DropStashTable("test-stash")
		require.NoError(t, err)

		exists, err := cache.TableExists("test-stash")
		require.NoError(t, err)
		assert.False(t, exists)

		_, err = cache.GetStash("test-stash")
		assert.ErrorIs(t, err, model.ErrStashNotFound)
	})
}

func TestSQLiteCache_ListStashes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stashes := []*model.Stash{
		{Name: "alpha", Prefix: "al-", Created: time.Now(), CreatedBy: "user"},
		{Name: "beta", Prefix: "be-", Created: time.Now(), CreatedBy: "user"},
		{Name: "gamma", Prefix: "ga-", Created: time.Now(), CreatedBy: "user"},
	}

	for _, s := range stashes {
		err := cache.CreateStashTable(s)
		require.NoError(t, err)
	}

	t.Run("list all stashes", func(t *testing.T) {
		result, err := cache.ListStashes()
		require.NoError(t, err)
		assert.Len(t, result, 3)

		// Should be sorted by name
		assert.Equal(t, "alpha", result[0].Name)
		assert.Equal(t, "beta", result[1].Name)
		assert.Equal(t, "gamma", result[2].Name)
	})
}

func TestSQLiteCache_GetNextChildSeq(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	now := time.Now()
	columns := []string{}

	// Insert parent record
	parent := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{},
	}
	err = cache.UpsertRecord("test-stash", parent, columns)
	require.NoError(t, err)

	t.Run("first child seq is 1", func(t *testing.T) {
		seq, err := cache.GetNextChildSeq("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Equal(t, 1, seq)
	})

	t.Run("seq increments after adding child", func(t *testing.T) {
		child1 := &model.Record{
			ID:        "ts-abc1.1",
			Hash:      "hash2",
			ParentID:  "ts-abc1",
			CreatedAt: now,
			CreatedBy: "user",
			UpdatedAt: now,
			UpdatedBy: "user",
			Fields:    map[string]interface{}{},
		}
		err = cache.UpsertRecord("test-stash", child1, columns)
		require.NoError(t, err)

		seq, err := cache.GetNextChildSeq("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Equal(t, 2, seq)
	})

	t.Run("seq handles gaps", func(t *testing.T) {
		child3 := &model.Record{
			ID:        "ts-abc1.3",
			Hash:      "hash3",
			ParentID:  "ts-abc1",
			CreatedAt: now,
			CreatedBy: "user",
			UpdatedAt: now,
			UpdatedBy: "user",
			Fields:    map[string]interface{}{},
		}
		err = cache.UpsertRecord("test-stash", child3, columns)
		require.NoError(t, err)

		seq, err := cache.GetNextChildSeq("test-stash", "ts-abc1")
		require.NoError(t, err)
		assert.Equal(t, 4, seq)
	})
}

func TestSQLiteCache_StashWithHyphen(t *testing.T) {
	// Test that stash names with hyphens work correctly
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "my-test-stash",
		Prefix:    "mts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "title", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"title"}
	now := time.Now()

	record := &model.Record{
		ID:        "mts-abc1",
		Hash:      "hash1",
		CreatedAt: now,
		CreatedBy: "user",
		UpdatedAt: now,
		UpdatedBy: "user",
		Fields:    map[string]interface{}{"title": "Test"},
	}

	err = cache.UpsertRecord("my-test-stash", record, columns)
	require.NoError(t, err)

	retrieved, err := cache.GetRecord("my-test-stash", "mts-abc1", columns)
	require.NoError(t, err)
	assert.Equal(t, "Test", retrieved.Fields["title"])
}

func TestSQLiteCache_WhereConditions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
			{Name: "price", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"name", "price"}
	now := time.Now()

	// Insert test records
	records := []*model.Record{
		{
			ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user",
			UpdatedAt: now, UpdatedBy: "user",
			Fields: map[string]interface{}{"name": "Apple", "price": "100"},
		},
		{
			ID: "ts-abc2", Hash: "hash2", CreatedAt: now.Add(time.Second), CreatedBy: "user",
			UpdatedAt: now.Add(time.Second), UpdatedBy: "user",
			Fields: map[string]interface{}{"name": "Banana", "price": "50"},
		},
		{
			ID: "ts-abc3", Hash: "hash3", CreatedAt: now.Add(2 * time.Second), CreatedBy: "user",
			UpdatedAt: now.Add(2 * time.Second), UpdatedBy: "user",
			Fields: map[string]interface{}{"name": "Cherry", "price": "150"},
		},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("filter by equality", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Where: []WhereCondition{
				{Field: "name", Operator: "=", Value: "Apple"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1", result[0].ID)
	})

	t.Run("filter by not equal", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Where: []WhereCondition{
				{Field: "name", Operator: "!=", Value: "Apple"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by LIKE", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Where: []WhereCondition{
				{Field: "name", Operator: "LIKE", Value: "%an%"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "Banana", result[0].Fields["name"])
	})

	t.Run("filter by greater than", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Where: []WhereCondition{
				{Field: "price", Operator: ">", Value: "75"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by less than", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Where: []WhereCondition{
				{Field: "price", Operator: "<", Value: "100"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "Banana", result[0].Fields["name"])
	})
}

func TestSQLiteCache_SearchFilter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "title", Added: time.Now(), AddedBy: "test-user"},
			{Name: "description", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"title", "description"}
	now := time.Now()

	records := []*model.Record{
		{
			ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user",
			UpdatedAt: now, UpdatedBy: "user",
			Fields: map[string]interface{}{"title": "Important Document", "description": "Contains vital info"},
		},
		{
			ID: "ts-abc2", Hash: "hash2", CreatedAt: now.Add(time.Second), CreatedBy: "user",
			UpdatedAt: now.Add(time.Second), UpdatedBy: "user",
			Fields: map[string]interface{}{"title": "Meeting Notes", "description": "From important meeting"},
		},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("search finds in title", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Search:   "Document",
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1", result[0].ID)
	})

	t.Run("search finds in description", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Search:   "vital",
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("search finds across multiple fields", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Search:   "important",
		})
		require.NoError(t, err)
		assert.Len(t, result, 2) // Both have "important"
	})

	t.Run("search finds in ID", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			Search:   "abc1",
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc1", result[0].ID)
	})
}

func TestSQLiteCache_OrderBy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "name", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"name"}
	now := time.Now()

	records := []*model.Record{
		{ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user", UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{"name": "Zebra"}},
		{ID: "ts-abc2", Hash: "hash2", CreatedAt: now.Add(time.Second), CreatedBy: "user", UpdatedAt: now.Add(time.Second), UpdatedBy: "user", Fields: map[string]interface{}{"name": "Apple"}},
		{ID: "ts-abc3", Hash: "hash3", CreatedAt: now.Add(2 * time.Second), CreatedBy: "user", UpdatedAt: now.Add(2 * time.Second), UpdatedBy: "user", Fields: map[string]interface{}{"name": "Mango"}},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("order by user field ascending", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			OrderBy:  "name",
		})
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, "Apple", result[0].Fields["name"])
		assert.Equal(t, "Mango", result[1].Fields["name"])
		assert.Equal(t, "Zebra", result[2].Fields["name"])
	})

	t.Run("order by user field descending", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID:   "*",
			OrderBy:    "name",
			Descending: true,
		})
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, "Zebra", result[0].Fields["name"])
		assert.Equal(t, "Mango", result[1].Fields["name"])
		assert.Equal(t, "Apple", result[2].Fields["name"])
	})

	t.Run("order by system field", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID: "*",
			OrderBy:  "id",
		})
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, "ts-abc1", result[0].ID)
	})
}

func TestSQLiteCache_DeletedOnlyFilter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{}
	now := time.Now()
	deletedAt := now.Add(time.Hour)

	records := []*model.Record{
		{ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user", UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{}},
		{ID: "ts-abc2", Hash: "hash2", CreatedAt: now, CreatedBy: "user", UpdatedAt: deletedAt, UpdatedBy: "deleter", DeletedAt: &deletedAt, DeletedBy: "deleter", Fields: map[string]interface{}{}},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("DeletedOnly returns only deleted records", func(t *testing.T) {
		result, err := cache.ListRecords("test-stash", columns, ListOptions{
			ParentID:    "*",
			DeletedOnly: true,
		})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ts-abc2", result[0].ID)
	})
}

func TestSQLiteCache_RawQuery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns: model.ColumnList{
			{Name: "value", Added: time.Now(), AddedBy: "test-user"},
		},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{"value"}
	now := time.Now()

	record := &model.Record{
		ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user",
		UpdatedAt: now, UpdatedBy: "user",
		Fields: map[string]interface{}{"value": "test-data"},
	}
	err = cache.UpsertRecord("test-stash", record, columns)
	require.NoError(t, err)

	t.Run("executes raw SELECT query", func(t *testing.T) {
		rows, cols, err := cache.RawQuery(`SELECT id, "value" FROM "test_stash"`)
		require.NoError(t, err)
		assert.Contains(t, cols, "id")
		assert.Contains(t, cols, "value")
		assert.Len(t, rows, 1)
		assert.Equal(t, "ts-abc1", rows[0]["id"])
	})

	t.Run("returns error for invalid query", func(t *testing.T) {
		_, _, err := cache.RawQuery("SELECT * FROM nonexistent_table")
		assert.Error(t, err)
	})
}

func TestSQLiteCache_CountRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}

	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	columns := []string{}
	now := time.Now()
	deletedAt := now.Add(time.Hour)

	// Add 3 records, 1 deleted
	records := []*model.Record{
		{ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user", UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{}},
		{ID: "ts-abc2", Hash: "hash2", CreatedAt: now, CreatedBy: "user", UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{}},
		{ID: "ts-abc3", Hash: "hash3", CreatedAt: now, CreatedBy: "user", UpdatedAt: deletedAt, UpdatedBy: "deleter", DeletedAt: &deletedAt, DeletedBy: "deleter", Fields: map[string]interface{}{}},
	}

	for _, r := range records {
		err := cache.UpsertRecord("test-stash", r, columns)
		require.NoError(t, err)
	}

	t.Run("counts non-deleted records", func(t *testing.T) {
		count, err := cache.CountRecords("test-stash")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestSQLiteCache_GetLastSyncTime(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("returns zero time when no stashes exist", func(t *testing.T) {
		syncTime, err := cache.GetLastSyncTime()
		require.NoError(t, err)
		assert.True(t, syncTime.IsZero())
	})

	t.Run("returns time after creating stash", func(t *testing.T) {
		stash := &model.Stash{
			Name:      "test-stash",
			Prefix:    "ts-",
			Created:   time.Now(),
			CreatedBy: "test-user",
			Columns:   model.ColumnList{},
		}
		err = cache.CreateStashTable(stash)
		require.NoError(t, err)

		syncTime, err := cache.GetLastSyncTime()
		require.NoError(t, err)
		assert.False(t, syncTime.IsZero())
	})
}

func TestSQLiteCache_TableExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	t.Run("returns false for nonexistent table", func(t *testing.T) {
		exists, err := cache.TableExists("nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns true after creating table", func(t *testing.T) {
		stash := &model.Stash{
			Name:      "test-stash",
			Prefix:    "ts-",
			Created:   time.Now(),
			CreatedBy: "test-user",
			Columns:   model.ColumnList{},
		}
		err = cache.CreateStashTable(stash)
		require.NoError(t, err)

		exists, err := cache.TableExists("test-stash")
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestSQLiteCache_ClearTable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}
	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	now := time.Now()
	record := &model.Record{
		ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user",
		UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{},
	}
	err = cache.UpsertRecord("test-stash", record, []string{})
	require.NoError(t, err)

	t.Run("clears all records", func(t *testing.T) {
		err := cache.ClearTable("test-stash")
		require.NoError(t, err)

		count, err := cache.CountRecords("test-stash")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestSQLiteCache_DeleteRecord(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cache, err := NewSQLiteCache(tmpDir)
	require.NoError(t, err)
	defer cache.Close()

	stash := &model.Stash{
		Name:      "test-stash",
		Prefix:    "ts-",
		Created:   time.Now(),
		CreatedBy: "test-user",
		Columns:   model.ColumnList{},
	}
	err = cache.CreateStashTable(stash)
	require.NoError(t, err)

	now := time.Now()
	record := &model.Record{
		ID: "ts-abc1", Hash: "hash1", CreatedAt: now, CreatedBy: "user",
		UpdatedAt: now, UpdatedBy: "user", Fields: map[string]interface{}{},
	}
	err = cache.UpsertRecord("test-stash", record, []string{})
	require.NoError(t, err)

	t.Run("deletes record from cache", func(t *testing.T) {
		err := cache.DeleteRecord("test-stash", "ts-abc1")
		require.NoError(t, err)

		_, err = cache.GetRecord("test-stash", "ts-abc1", []string{})
		assert.ErrorIs(t, err, model.ErrRecordNotFound)
	})
}

func TestSanitizeTableName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-stash", "my_stash"},
		{"my-test-stash", "my_test_stash"},
		{"simple", "simple"},
		{"already_underscored", "already_underscored"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeTableName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
