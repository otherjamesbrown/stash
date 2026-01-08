package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJSONOutput(t *testing.T) {
	t.Run("parses valid JSON array", func(t *testing.T) {
		input := `[{"name": "foo", "value": 1}, {"name": "bar", "value": 2}]`

		result := ParseJSONOutput(t, input)

		assert.Len(t, result, 2)
		assert.Equal(t, "foo", result[0]["name"])
		assert.Equal(t, float64(1), result[0]["value"])
	})

	t.Run("parses empty array", func(t *testing.T) {
		input := `[]`

		result := ParseJSONOutput(t, input)

		assert.Empty(t, result)
	})
}

func TestParseJSONObject(t *testing.T) {
	t.Run("parses valid JSON object", func(t *testing.T) {
		input := `{"name": "test", "count": 42, "active": true}`

		result := ParseJSONObject(t, input)

		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(42), result["count"])
		assert.Equal(t, true, result["active"])
	})

	t.Run("parses nested object", func(t *testing.T) {
		input := `{"outer": {"inner": "value"}}`

		result := ParseJSONObject(t, input)

		outer := result["outer"].(map[string]interface{})
		assert.Equal(t, "value", outer["inner"])
	})
}

func TestGetField(t *testing.T) {
	obj := map[string]interface{}{
		"name":  "test",
		"count": float64(42),
	}

	t.Run("returns string field", func(t *testing.T) {
		assert.Equal(t, "test", GetField(obj, "name"))
	})

	t.Run("returns empty for non-existent field", func(t *testing.T) {
		assert.Equal(t, "", GetField(obj, "nonexistent"))
	})

	t.Run("returns string representation of numeric field", func(t *testing.T) {
		assert.Equal(t, "42", GetField(obj, "count"))
	})
}

func TestGetFieldFloat(t *testing.T) {
	obj := map[string]interface{}{
		"name":  "test",
		"count": float64(42),
		"price": float64(19.99),
	}

	t.Run("returns float field", func(t *testing.T) {
		assert.Equal(t, float64(42), GetFieldFloat(obj, "count"))
		assert.Equal(t, float64(19.99), GetFieldFloat(obj, "price"))
	})

	t.Run("returns 0 for non-existent field", func(t *testing.T) {
		assert.Equal(t, float64(0), GetFieldFloat(obj, "nonexistent"))
	})

	t.Run("returns 0 for non-float field", func(t *testing.T) {
		assert.Equal(t, float64(0), GetFieldFloat(obj, "name"))
	})
}

func TestGetFieldBool(t *testing.T) {
	obj := map[string]interface{}{
		"active":   true,
		"deleted":  false,
		"name":     "test",
	}

	t.Run("returns true for true field", func(t *testing.T) {
		assert.True(t, GetFieldBool(obj, "active"))
	})

	t.Run("returns false for false field", func(t *testing.T) {
		assert.False(t, GetFieldBool(obj, "deleted"))
	})

	t.Run("returns false for non-existent field", func(t *testing.T) {
		assert.False(t, GetFieldBool(obj, "nonexistent"))
	})

	t.Run("returns false for non-bool field", func(t *testing.T) {
		assert.False(t, GetFieldBool(obj, "name"))
	})
}

func TestGetNestedField(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"version": "1.0",
		},
		"name": "test",
	}

	t.Run("returns nested map", func(t *testing.T) {
		nested := GetNestedField(obj, "metadata")
		assert.NotNil(t, nested)
		assert.Equal(t, "1.0", nested["version"])
	})

	t.Run("returns nil for non-existent field", func(t *testing.T) {
		assert.Nil(t, GetNestedField(obj, "nonexistent"))
	})

	t.Run("returns nil for non-map field", func(t *testing.T) {
		assert.Nil(t, GetNestedField(obj, "name"))
	})
}

func TestGetArrayField(t *testing.T) {
	obj := map[string]interface{}{
		"tags": []interface{}{"a", "b", "c"},
		"name": "test",
	}

	t.Run("returns array field", func(t *testing.T) {
		arr := GetArrayField(obj, "tags")
		assert.Len(t, arr, 3)
		assert.Equal(t, "a", arr[0])
	})

	t.Run("returns nil for non-existent field", func(t *testing.T) {
		assert.Nil(t, GetArrayField(obj, "nonexistent"))
	})

	t.Run("returns nil for non-array field", func(t *testing.T) {
		assert.Nil(t, GetArrayField(obj, "name"))
	})
}
