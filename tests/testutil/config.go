package testutil

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

// ReadConfig reads and parses .stash/<name>/config.json.
func ReadConfig(t *testing.T, dir, stashName string) map[string]interface{} {
	t.Helper()

	configPath := ConfigPath(dir, stashName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file %s: %v", configPath, err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse config JSON: %v", err)
	}

	return config
}

// ReadJSONL reads all records from records.jsonl.
// Returns an empty slice if the file doesn't exist or is empty.
func ReadJSONL(t *testing.T, dir, stashName string) []map[string]interface{} {
	t.Helper()

	recordsPath := RecordsPath(dir, stashName)
	file, err := os.Open(recordsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}
		}
		t.Fatalf("failed to open records file %s: %v", recordsPath, err)
	}
	defer file.Close()

	var records []map[string]interface{}
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("failed to parse JSONL line %d: %v\nline: %s", lineNum, err, line)
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading records file: %v", err)
	}

	return records
}

// GetColumns extracts the columns array from a parsed config.
func GetColumns(config map[string]interface{}) []map[string]interface{} {
	cols := GetArrayField(config, "columns")
	if cols == nil {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(cols))
	for _, c := range cols {
		if col, ok := c.(map[string]interface{}); ok {
			result = append(result, col)
		}
	}
	return result
}

// GetColumnNames extracts just the column names from a parsed config.
func GetColumnNames(config map[string]interface{}) []string {
	cols := GetColumns(config)
	if cols == nil {
		return nil
	}

	names := make([]string, 0, len(cols))
	for _, col := range cols {
		if name := GetField(col, "name"); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// FindRecordByID finds a record in the JSONL data by its _id field.
// Returns nil if not found.
func FindRecordByID(records []map[string]interface{}, id string) map[string]interface{} {
	for _, rec := range records {
		if GetField(rec, "_id") == id {
			return rec
		}
	}
	return nil
}

// FilterActiveRecords returns only records that are not soft-deleted.
// A record is considered deleted if it has "_deleted": true.
func FilterActiveRecords(records []map[string]interface{}) []map[string]interface{} {
	var active []map[string]interface{}
	for _, rec := range records {
		if !GetFieldBool(rec, "_deleted") {
			active = append(active, rec)
		}
	}
	return active
}

// FilterDeletedRecords returns only records that are soft-deleted.
func FilterDeletedRecords(records []map[string]interface{}) []map[string]interface{} {
	var deleted []map[string]interface{}
	for _, rec := range records {
		if GetFieldBool(rec, "_deleted") {
			deleted = append(deleted, rec)
		}
	}
	return deleted
}
