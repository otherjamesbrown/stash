package testutil

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ParseJSONOutput parses JSON array output into []map[string]interface{}.
func ParseJSONOutput(t *testing.T, output string) []map[string]interface{} {
	t.Helper()

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON array: %v\noutput: %s", err, output)
	}
	return result
}

// ParseJSONObject parses single JSON object output.
func ParseJSONObject(t *testing.T, output string) map[string]interface{} {
	t.Helper()

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON object: %v\noutput: %s", err, output)
	}
	return result
}

// ShowRecord returns parsed record by running stash show --json.
func ShowRecord(t *testing.T, dir, id string) map[string]interface{} {
	t.Helper()

	result := MustSucceedInDir(t, dir, "show", id, "--json")
	return ParseJSONObject(t, result.Stdout)
}

// ListRecords returns parsed records by running stash list --json.
func ListRecords(t *testing.T, dir string) []map[string]interface{} {
	t.Helper()

	result := MustSucceedInDir(t, dir, "list", "--json")
	return ParseJSONOutput(t, result.Stdout)
}

// GetField extracts a string field from a parsed JSON object.
// Handles both string and numeric values, converting them to strings.
// Returns empty string if the field doesn't exist.
func GetField(obj map[string]interface{}, key string) string {
	if val, ok := obj[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case float64:
			// Convert float to string, handling integers properly
			if v == float64(int64(v)) {
				return fmt.Sprintf("%d", int64(v))
			}
			return fmt.Sprintf("%g", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// GetFieldFloat extracts a float64 field from a parsed JSON object.
// Returns 0 if the field doesn't exist or is not a number.
func GetFieldFloat(obj map[string]interface{}, key string) float64 {
	if val, ok := obj[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
	}
	return 0
}

// GetFieldBool extracts a bool field from a parsed JSON object.
// Returns false if the field doesn't exist or is not a bool.
func GetFieldBool(obj map[string]interface{}, key string) bool {
	if val, ok := obj[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetNestedField extracts a nested map field from a parsed JSON object.
// Returns nil if the field doesn't exist or is not a map.
func GetNestedField(obj map[string]interface{}, key string) map[string]interface{} {
	if val, ok := obj[key]; ok {
		if nested, ok := val.(map[string]interface{}); ok {
			return nested
		}
	}
	return nil
}

// GetArrayField extracts an array field from a parsed JSON object.
// Returns nil if the field doesn't exist or is not an array.
func GetArrayField(obj map[string]interface{}, key string) []interface{} {
	if val, ok := obj[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			return arr
		}
	}
	return nil
}

// ParseFilesOutput parses the JSON output from the files command.
// The files command returns an object with a "files" array.
// Returns the files as []map[string]interface{}.
func ParseFilesOutput(t *testing.T, output string) []map[string]interface{} {
	t.Helper()

	obj := ParseJSONObject(t, output)
	filesArr := GetArrayField(obj, "files")
	if filesArr == nil {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0, len(filesArr))
	for _, item := range filesArr {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}
