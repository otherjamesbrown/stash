package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateHash(t *testing.T) {
	t.Run("returns 12 character hash", func(t *testing.T) {
		hash := CalculateHash(map[string]interface{}{"Name": "Test"})
		assert.Len(t, hash, 12)
	})

	t.Run("same fields produce same hash", func(t *testing.T) {
		fields1 := map[string]interface{}{"Name": "Test", "Value": 123}
		fields2 := map[string]interface{}{"Name": "Test", "Value": 123}
		assert.Equal(t, CalculateHash(fields1), CalculateHash(fields2))
	})

	t.Run("different fields produce different hash", func(t *testing.T) {
		fields1 := map[string]interface{}{"Name": "Test", "Value": 123}
		fields2 := map[string]interface{}{"Name": "Test", "Value": 456}
		assert.NotEqual(t, CalculateHash(fields1), CalculateHash(fields2))
	})

	t.Run("field order does not affect hash", func(t *testing.T) {
		fields1 := map[string]interface{}{"A": 1, "B": 2, "C": 3}
		fields2 := map[string]interface{}{"C": 3, "A": 1, "B": 2}
		assert.Equal(t, CalculateHash(fields1), CalculateHash(fields2))
	})

	t.Run("ignores system fields", func(t *testing.T) {
		fields1 := map[string]interface{}{"Name": "Test", "_id": "inv-ex4j"}
		fields2 := map[string]interface{}{"Name": "Test", "_id": "inv-8t5n"}
		assert.Equal(t, CalculateHash(fields1), CalculateHash(fields2))
	})

	t.Run("empty fields produce consistent hash", func(t *testing.T) {
		hash1 := CalculateHash(map[string]interface{}{})
		hash2 := CalculateHash(map[string]interface{}{})
		assert.Equal(t, hash1, hash2)
	})
}

func TestRecordIsDeleted(t *testing.T) {
	t.Run("returns false when not deleted", func(t *testing.T) {
		r := &Record{}
		assert.False(t, r.IsDeleted())
	})

	t.Run("returns true when deleted", func(t *testing.T) {
		now := time.Now()
		r := &Record{DeletedAt: &now}
		assert.True(t, r.IsDeleted())
	})
}

func TestRecordJSON(t *testing.T) {
	t.Run("marshal flattens fields", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		r := &Record{
			ID:        "inv-ex4j",
			Hash:      "abc123def456",
			Operation: OpCreate,
			CreatedAt: now,
			CreatedBy: "alice",
			UpdatedAt: now,
			UpdatedBy: "alice",
			Branch:    "main",
			Fields: map[string]interface{}{
				"Name":  "Laptop",
				"Price": 999,
			},
		}

		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))

		assert.Equal(t, "inv-ex4j", m["_id"])
		assert.Equal(t, "abc123def456", m["_hash"])
		assert.Equal(t, "Laptop", m["Name"])
		assert.Equal(t, float64(999), m["Price"])
	})

	t.Run("unmarshal extracts fields", func(t *testing.T) {
		data := []byte(`{
			"_id": "inv-ex4j",
			"_hash": "abc123def456",
			"_op": "create",
			"_created_at": "2025-01-08T10:00:00Z",
			"_created_by": "alice",
			"_updated_at": "2025-01-08T10:00:00Z",
			"_updated_by": "alice",
			"_branch": "main",
			"Name": "Laptop",
			"Price": 999
		}`)

		var r Record
		require.NoError(t, json.Unmarshal(data, &r))

		assert.Equal(t, "inv-ex4j", r.ID)
		assert.Equal(t, "abc123def456", r.Hash)
		assert.Equal(t, OpCreate, r.Operation)
		assert.Equal(t, "alice", r.CreatedBy)
		assert.Equal(t, "main", r.Branch)
		assert.Equal(t, "Laptop", r.Fields["Name"])
		assert.Equal(t, float64(999), r.Fields["Price"])
	})

	t.Run("roundtrip preserves data", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		original := &Record{
			ID:        "inv-ex4j",
			Hash:      "abc123def456",
			ParentID:  "inv-parent",
			Operation: OpUpdate,
			CreatedAt: now,
			CreatedBy: "alice",
			UpdatedAt: now,
			UpdatedBy: "bob",
			Branch:    "feature",
			Fields: map[string]interface{}{
				"Name":     "Laptop",
				"Price":    999,
				"InStock":  true,
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored Record
		require.NoError(t, json.Unmarshal(data, &restored))

		assert.Equal(t, original.ID, restored.ID)
		assert.Equal(t, original.Hash, restored.Hash)
		assert.Equal(t, original.ParentID, restored.ParentID)
		assert.Equal(t, original.Operation, restored.Operation)
		assert.Equal(t, original.CreatedBy, restored.CreatedBy)
		assert.Equal(t, original.UpdatedBy, restored.UpdatedBy)
		assert.Equal(t, original.Branch, restored.Branch)
		assert.Equal(t, original.Fields["Name"], restored.Fields["Name"])
	})
}

func TestRecordGetField(t *testing.T) {
	r := &Record{
		Fields: map[string]interface{}{
			"Name":  "Laptop",
			"Price": 999,
		},
	}

	t.Run("exact match", func(t *testing.T) {
		v, ok := r.GetField("Name")
		assert.True(t, ok)
		assert.Equal(t, "Laptop", v)
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		v, ok := r.GetField("name")
		assert.True(t, ok)
		assert.Equal(t, "Laptop", v)

		v, ok = r.GetField("NAME")
		assert.True(t, ok)
		assert.Equal(t, "Laptop", v)
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := r.GetField("NonExistent")
		assert.False(t, ok)
	})
}

func TestRecordSetField(t *testing.T) {
	t.Run("sets new field", func(t *testing.T) {
		r := &Record{}
		r.SetField("Name", "Laptop")
		assert.Equal(t, "Laptop", r.Fields["Name"])
	})

	t.Run("updates existing field preserving case", func(t *testing.T) {
		r := &Record{
			Fields: map[string]interface{}{"Name": "Laptop"},
		}
		r.SetField("name", "Phone")
		assert.Equal(t, "Phone", r.Fields["Name"])
		assert.Nil(t, r.Fields["name"]) // Should use original case
	})
}

func TestRecordCalculateHash_EdgeCases(t *testing.T) {
	t.Run("nil fields produces same hash as empty", func(t *testing.T) {
		hash1 := CalculateHash(nil)
		hash2 := CalculateHash(map[string]interface{}{})
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different value types produce different hashes", func(t *testing.T) {
		hash1 := CalculateHash(map[string]interface{}{"value": "42"})
		hash2 := CalculateHash(map[string]interface{}{"value": 42})
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("nested objects produce consistent hash", func(t *testing.T) {
		fields := map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
		}
		hash1 := CalculateHash(fields)
		hash2 := CalculateHash(fields)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("arrays produce consistent hash", func(t *testing.T) {
		fields := map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
		}
		hash1 := CalculateHash(fields)
		hash2 := CalculateHash(fields)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("boolean values produce consistent hash", func(t *testing.T) {
		hash1 := CalculateHash(map[string]interface{}{"flag": true})
		hash2 := CalculateHash(map[string]interface{}{"flag": true})
		hash3 := CalculateHash(map[string]interface{}{"flag": false})
		assert.Equal(t, hash1, hash2)
		assert.NotEqual(t, hash1, hash3)
	})

	t.Run("null values handled correctly", func(t *testing.T) {
		hash1 := CalculateHash(map[string]interface{}{"value": nil})
		hash2 := CalculateHash(map[string]interface{}{"value": nil})
		assert.Equal(t, hash1, hash2)
	})

	t.Run("Record.CalculateHash uses only user fields", func(t *testing.T) {
		r := &Record{
			ID:   "ts-abc1",
			Hash: "oldhash",
			Fields: map[string]interface{}{
				"Name": "Test",
			},
		}
		hash := r.CalculateHash()
		expectedHash := CalculateHash(map[string]interface{}{"Name": "Test"})
		assert.Equal(t, expectedHash, hash)
	})
}

func TestRecordJSON_EdgeCases(t *testing.T) {
	t.Run("marshal with deleted record", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		deletedAt := now.Add(time.Hour)
		r := &Record{
			ID:        "ts-abc1",
			Hash:      "hash123",
			Operation: OpDelete,
			CreatedAt: now,
			CreatedBy: "creator",
			UpdatedAt: deletedAt,
			UpdatedBy: "deleter",
			DeletedAt: &deletedAt,
			DeletedBy: "deleter",
			Fields:    map[string]interface{}{},
		}

		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))

		assert.NotNil(t, m["_deleted_at"])
		assert.Equal(t, "deleter", m["_deleted_by"])
	})

	t.Run("unmarshal with missing optional fields", func(t *testing.T) {
		data := []byte(`{
			"_id": "ts-abc1",
			"_hash": "hash123",
			"_op": "create",
			"_created_at": "2025-01-08T10:00:00Z",
			"_created_by": "alice",
			"_updated_at": "2025-01-08T10:00:00Z",
			"_updated_by": "alice"
		}`)

		var r Record
		require.NoError(t, json.Unmarshal(data, &r))

		assert.Equal(t, "ts-abc1", r.ID)
		assert.Empty(t, r.ParentID)
		assert.Empty(t, r.Branch)
		assert.Nil(t, r.DeletedAt)
		assert.Empty(t, r.DeletedBy)
	})

	t.Run("unmarshal with special characters in fields", func(t *testing.T) {
		data := []byte(`{
			"_id": "ts-abc1",
			"_hash": "hash123",
			"_op": "create",
			"_created_at": "2025-01-08T10:00:00Z",
			"_created_by": "alice",
			"_updated_at": "2025-01-08T10:00:00Z",
			"_updated_by": "alice",
			"Name": "Test \"quoted\" value",
			"Description": "Line1\nLine2"
		}`)

		var r Record
		require.NoError(t, json.Unmarshal(data, &r))

		assert.Equal(t, "Test \"quoted\" value", r.Fields["Name"])
		assert.Equal(t, "Line1\nLine2", r.Fields["Description"])
	})

	t.Run("unmarshal invalid JSON returns error", func(t *testing.T) {
		data := []byte(`{invalid json}`)
		var r Record
		err := json.Unmarshal(data, &r)
		assert.Error(t, err)
	})
}

func TestRecordGetField_EdgeCases(t *testing.T) {
	t.Run("returns false for nil fields", func(t *testing.T) {
		r := &Record{Fields: nil}
		_, ok := r.GetField("Name")
		assert.False(t, ok)
	})

	t.Run("returns value with exact case match preferred", func(t *testing.T) {
		r := &Record{
			Fields: map[string]interface{}{
				"Name": "Exact",
			},
		}
		v, ok := r.GetField("Name")
		assert.True(t, ok)
		assert.Equal(t, "Exact", v)
	})
}

func TestRecordSetField_EdgeCases(t *testing.T) {
	t.Run("sets field on nil Fields map", func(t *testing.T) {
		r := &Record{Fields: nil}
		r.SetField("Name", "Test")
		assert.NotNil(t, r.Fields)
		assert.Equal(t, "Test", r.Fields["Name"])
	})

	t.Run("preserves case across multiple updates", func(t *testing.T) {
		r := &Record{}
		r.SetField("OriginalCase", "first")
		r.SetField("ORIGINALCASE", "second")
		r.SetField("originalcase", "third")

		// All should update the same key
		assert.Len(t, r.Fields, 1)
		assert.Equal(t, "third", r.Fields["OriginalCase"])
	})

	t.Run("sets different values for different types", func(t *testing.T) {
		r := &Record{}
		r.SetField("string", "hello")
		r.SetField("number", 42)
		r.SetField("bool", true)
		r.SetField("nil", nil)

		assert.Equal(t, "hello", r.Fields["string"])
		assert.Equal(t, 42, r.Fields["number"])
		assert.Equal(t, true, r.Fields["bool"])
		assert.Nil(t, r.Fields["nil"])
	})
}

func TestRecordOperationConstants(t *testing.T) {
	t.Run("operation constants have expected values", func(t *testing.T) {
		assert.Equal(t, "create", OpCreate)
		assert.Equal(t, "update", OpUpdate)
		assert.Equal(t, "delete", OpDelete)
		assert.Equal(t, "restore", OpRestore)
	})
}

func TestRecordIsDeleted_EdgeCases(t *testing.T) {
	t.Run("zero time pointer is still deleted", func(t *testing.T) {
		zeroTime := time.Time{}
		r := &Record{DeletedAt: &zeroTime}
		// A non-nil pointer means deleted, even if zero value
		assert.True(t, r.IsDeleted())
	})
}
