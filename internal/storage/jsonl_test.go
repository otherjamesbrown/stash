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

func TestJSONLStore_AppendAndRead(t *testing.T) {
	// Setup temp directory
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	t.Run("read empty returns empty slice", func(t *testing.T) {
		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("append single record", func(t *testing.T) {
		record := &model.Record{
			ID:        "ts-abc1",
			Hash:      "hash123456",
			CreatedAt: time.Now(),
			CreatedBy: "test-user",
			UpdatedAt: time.Now(),
			UpdatedBy: "test-user",
			Operation: model.OpCreate,
			Fields: map[string]interface{}{
				"name":  "Test Item",
				"value": float64(42),
			},
		}

		err := store.AppendRecord(stashName, record)
		require.NoError(t, err)

		// Verify file was created
		recordsPath := filepath.Join(tmpDir, stashName, "records.jsonl")
		assert.FileExists(t, recordsPath)
	})

	t.Run("read appended record", func(t *testing.T) {
		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, records, 1)

		assert.Equal(t, "ts-abc1", records[0].ID)
		assert.Equal(t, "Test Item", records[0].Fields["name"])
		assert.Equal(t, float64(42), records[0].Fields["value"])
		assert.Equal(t, model.OpCreate, records[0].Operation)
	})

	t.Run("append multiple records", func(t *testing.T) {
		record2 := &model.Record{
			ID:        "ts-abc2",
			Hash:      "hash234567",
			CreatedAt: time.Now(),
			CreatedBy: "test-user",
			UpdatedAt: time.Now(),
			UpdatedBy: "test-user",
			Operation: model.OpCreate,
			Fields: map[string]interface{}{
				"name": "Second Item",
			},
		}

		err := store.AppendRecord(stashName, record2)
		require.NoError(t, err)

		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, records, 2)

		assert.Equal(t, "ts-abc1", records[0].ID)
		assert.Equal(t, "ts-abc2", records[1].ID)
	})

	t.Run("append update operation", func(t *testing.T) {
		updateRecord := &model.Record{
			ID:        "ts-abc1",
			Hash:      "hash345678",
			CreatedAt: time.Now(),
			CreatedBy: "test-user",
			UpdatedAt: time.Now(),
			UpdatedBy: "updater",
			Operation: model.OpUpdate,
			Fields: map[string]interface{}{
				"name":  "Updated Item",
				"value": float64(100),
			},
		}

		err := store.AppendRecord(stashName, updateRecord)
		require.NoError(t, err)

		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, records, 3)

		// Third record should be the update
		assert.Equal(t, "ts-abc1", records[2].ID)
		assert.Equal(t, model.OpUpdate, records[2].Operation)
		assert.Equal(t, "Updated Item", records[2].Fields["name"])
	})

	t.Run("append delete operation", func(t *testing.T) {
		now := time.Now()
		deleteRecord := &model.Record{
			ID:        "ts-abc1",
			Hash:      "hash345678",
			CreatedAt: time.Now(),
			CreatedBy: "test-user",
			UpdatedAt: now,
			UpdatedBy: "deleter",
			DeletedAt: &now,
			DeletedBy: "deleter",
			Operation: model.OpDelete,
			Fields: map[string]interface{}{
				"name":  "Updated Item",
				"value": float64(100),
			},
		}

		err := store.AppendRecord(stashName, deleteRecord)
		require.NoError(t, err)

		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, records, 4)

		assert.Equal(t, model.OpDelete, records[3].Operation)
		assert.NotNil(t, records[3].DeletedAt)
	})
}

func TestJSONLStore_WriteAllRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	// Create initial records
	records := []*model.Record{
		{
			ID:        "ts-abc1",
			Hash:      "hash1",
			CreatedAt: time.Now(),
			CreatedBy: "user1",
			UpdatedAt: time.Now(),
			UpdatedBy: "user1",
			Operation: model.OpCreate,
			Fields:    map[string]interface{}{"name": "First"},
		},
		{
			ID:        "ts-abc2",
			Hash:      "hash2",
			CreatedAt: time.Now(),
			CreatedBy: "user2",
			UpdatedAt: time.Now(),
			UpdatedBy: "user2",
			Operation: model.OpCreate,
			Fields:    map[string]interface{}{"name": "Second"},
		},
	}

	t.Run("write all records", func(t *testing.T) {
		err := store.WriteAllRecords(stashName, records)
		require.NoError(t, err)

		readRecords, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, readRecords, 2)
	})

	t.Run("overwrite all records", func(t *testing.T) {
		newRecords := []*model.Record{
			{
				ID:        "ts-xyz1",
				Hash:      "hash3",
				CreatedAt: time.Now(),
				CreatedBy: "user3",
				UpdatedAt: time.Now(),
				UpdatedBy: "user3",
				Operation: model.OpCreate,
				Fields:    map[string]interface{}{"name": "New Record"},
			},
		}

		err := store.WriteAllRecords(stashName, newRecords)
		require.NoError(t, err)

		readRecords, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		require.Len(t, readRecords, 1)
		assert.Equal(t, "ts-xyz1", readRecords[0].ID)
	})
}

func TestJSONLStore_Exists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	t.Run("does not exist initially", func(t *testing.T) {
		assert.False(t, store.Exists(stashName))
	})

	t.Run("exists after append", func(t *testing.T) {
		record := &model.Record{
			ID:        "ts-abc1",
			Hash:      "hash1",
			CreatedAt: time.Now(),
			CreatedBy: "user",
			UpdatedAt: time.Now(),
			UpdatedBy: "user",
			Operation: model.OpCreate,
			Fields:    map[string]interface{}{},
		}

		err := store.AppendRecord(stashName, record)
		require.NoError(t, err)

		assert.True(t, store.Exists(stashName))
	})
}

func TestJSONLStore_DeleteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	// Create a record first
	record := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash1",
		CreatedAt: time.Now(),
		CreatedBy: "user",
		UpdatedAt: time.Now(),
		UpdatedBy: "user",
		Operation: model.OpCreate,
		Fields:    map[string]interface{}{},
	}
	err = store.AppendRecord(stashName, record)
	require.NoError(t, err)
	require.True(t, store.Exists(stashName))

	t.Run("delete existing file", func(t *testing.T) {
		err := store.DeleteFile(stashName)
		require.NoError(t, err)
		assert.False(t, store.Exists(stashName))
	})

	t.Run("delete non-existent file succeeds", func(t *testing.T) {
		err := store.DeleteFile("nonexistent")
		require.NoError(t, err)
	})
}

func TestJSONLStore_RecordWithParent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	parentRecord := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash1",
		CreatedAt: time.Now(),
		CreatedBy: "user",
		UpdatedAt: time.Now(),
		UpdatedBy: "user",
		Operation: model.OpCreate,
		Fields:    map[string]interface{}{"name": "Parent"},
	}

	childRecord := &model.Record{
		ID:        "ts-abc1.1",
		Hash:      "hash2",
		ParentID:  "ts-abc1",
		CreatedAt: time.Now(),
		CreatedBy: "user",
		UpdatedAt: time.Now(),
		UpdatedBy: "user",
		Operation: model.OpCreate,
		Fields:    map[string]interface{}{"name": "Child"},
	}

	err = store.AppendRecord(stashName, parentRecord)
	require.NoError(t, err)

	err = store.AppendRecord(stashName, childRecord)
	require.NoError(t, err)

	records, err := store.ReadAllRecords(stashName)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, "", records[0].ParentID)
	assert.Equal(t, "ts-abc1", records[1].ParentID)
}

func TestJSONLStore_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	stashName := "test-stash"
	stashDir := filepath.Join(tmpDir, stashName)
	require.NoError(t, os.MkdirAll(stashDir, 0755))

	// Write invalid JSON directly to records.jsonl
	recordsPath := filepath.Join(stashDir, "records.jsonl")
	require.NoError(t, os.WriteFile(recordsPath, []byte("invalid json\n"), 0644))

	store := NewJSONLStore(tmpDir)

	t.Run("ReadAllRecords returns error on invalid JSON", func(t *testing.T) {
		_, err := store.ReadAllRecords(stashName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "line 1")
	})
}

func TestJSONLStore_EmptyLines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	stashName := "test-stash"
	stashDir := filepath.Join(tmpDir, stashName)
	require.NoError(t, os.MkdirAll(stashDir, 0755))

	// Write JSON with empty lines
	recordsPath := filepath.Join(stashDir, "records.jsonl")
	content := `{"_id":"ts-abc1","_hash":"hash1","_op":"create","_created_at":"2025-01-08T10:00:00Z","_created_by":"user","_updated_at":"2025-01-08T10:00:00Z","_updated_by":"user"}

{"_id":"ts-abc2","_hash":"hash2","_op":"create","_created_at":"2025-01-08T10:00:00Z","_created_by":"user","_updated_at":"2025-01-08T10:00:00Z","_updated_by":"user"}
`
	require.NoError(t, os.WriteFile(recordsPath, []byte(content), 0644))

	store := NewJSONLStore(tmpDir)

	t.Run("ReadAllRecords skips empty lines", func(t *testing.T) {
		records, err := store.ReadAllRecords(stashName)
		require.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

func TestJSONLStore_RecordWithBranch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	record := &model.Record{
		ID:        "ts-abc1",
		Hash:      "hash1",
		CreatedAt: time.Now(),
		CreatedBy: "user",
		UpdatedAt: time.Now(),
		UpdatedBy: "user",
		Branch:    "feature-branch",
		Operation: model.OpCreate,
		Fields:    map[string]interface{}{"name": "Test"},
	}

	err = store.AppendRecord(stashName, record)
	require.NoError(t, err)

	records, err := store.ReadAllRecords(stashName)
	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.Equal(t, "feature-branch", records[0].Branch)
}

func TestJSONLStore_WriteAllRecords_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stash-jsonl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewJSONLStore(tmpDir)
	stashName := "test-stash"

	// Write empty records array
	err = store.WriteAllRecords(stashName, []*model.Record{})
	require.NoError(t, err)

	// File should exist but be empty
	records, err := store.ReadAllRecords(stashName)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestJSONLStore_GetRecordsPath(t *testing.T) {
	store := NewJSONLStore("/base/dir")

	path := store.getRecordsPath("my-stash")
	assert.Equal(t, "/base/dir/my-stash/records.jsonl", path)
}
