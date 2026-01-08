package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStashDir(t *testing.T) {
	result := StashDir("/home/test", "inventory")
	assert.Equal(t, "/home/test/.stash/inventory", result)
}

func TestConfigPath(t *testing.T) {
	result := ConfigPath("/home/test", "inventory")
	assert.Equal(t, "/home/test/.stash/inventory/config.json", result)
}

func TestRecordsPath(t *testing.T) {
	result := RecordsPath("/home/test", "inventory")
	assert.Equal(t, "/home/test/.stash/inventory/records.jsonl", result)
}

func TestGetColumns(t *testing.T) {
	config := map[string]interface{}{
		"columns": []interface{}{
			map[string]interface{}{"name": "Name", "desc": "Item name"},
			map[string]interface{}{"name": "Price", "desc": "Price in USD"},
		},
	}

	t.Run("extracts columns from config", func(t *testing.T) {
		cols := GetColumns(config)
		assert.Len(t, cols, 2)
		assert.Equal(t, "Name", cols[0]["name"])
		assert.Equal(t, "Price", cols[1]["name"])
	})

	t.Run("returns nil for missing columns", func(t *testing.T) {
		emptyConfig := map[string]interface{}{}
		cols := GetColumns(emptyConfig)
		assert.Nil(t, cols)
	})
}

func TestGetColumnNames(t *testing.T) {
	config := map[string]interface{}{
		"columns": []interface{}{
			map[string]interface{}{"name": "Name"},
			map[string]interface{}{"name": "Price"},
			map[string]interface{}{"name": "Category"},
		},
	}

	t.Run("extracts column names", func(t *testing.T) {
		names := GetColumnNames(config)
		assert.Equal(t, []string{"Name", "Price", "Category"}, names)
	})

	t.Run("returns nil for missing columns", func(t *testing.T) {
		emptyConfig := map[string]interface{}{}
		names := GetColumnNames(emptyConfig)
		assert.Nil(t, names)
	})
}

func TestFindRecordByID(t *testing.T) {
	records := []map[string]interface{}{
		{"_id": "inv-001", "Name": "Widget"},
		{"_id": "inv-002", "Name": "Gadget"},
		{"_id": "inv-003", "Name": "Tool"},
	}

	t.Run("finds existing record", func(t *testing.T) {
		rec := FindRecordByID(records, "inv-002")
		assert.NotNil(t, rec)
		assert.Equal(t, "Gadget", rec["Name"])
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		rec := FindRecordByID(records, "inv-999")
		assert.Nil(t, rec)
	})
}

func TestFilterActiveRecords(t *testing.T) {
	records := []map[string]interface{}{
		{"_id": "inv-001", "Name": "Active1"},
		{"_id": "inv-002", "Name": "Deleted", "_deleted": true},
		{"_id": "inv-003", "Name": "Active2"},
		{"_id": "inv-004", "Name": "Deleted2", "_deleted": true},
	}

	active := FilterActiveRecords(records)

	assert.Len(t, active, 2)
	assert.Equal(t, "inv-001", active[0]["_id"])
	assert.Equal(t, "inv-003", active[1]["_id"])
}

func TestFilterDeletedRecords(t *testing.T) {
	records := []map[string]interface{}{
		{"_id": "inv-001", "Name": "Active1"},
		{"_id": "inv-002", "Name": "Deleted", "_deleted": true},
		{"_id": "inv-003", "Name": "Active2"},
		{"_id": "inv-004", "Name": "Deleted2", "_deleted": true},
	}

	deleted := FilterDeletedRecords(records)

	assert.Len(t, deleted, 2)
	assert.Equal(t, "inv-002", deleted[0]["_id"])
	assert.Equal(t, "inv-004", deleted[1]["_id"])
}
