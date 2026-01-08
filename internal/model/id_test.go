package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateID(t *testing.T) {
	t.Run("generates valid ID with prefix", func(t *testing.T) {
		id, err := GenerateID("inv-")
		require.NoError(t, err)
		assert.Regexp(t, `^inv-[0-9a-z]{4}$`, id)
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := GenerateID("ts-")
			require.NoError(t, err)
			assert.False(t, ids[id], "duplicate ID generated: %s", id)
			ids[id] = true
		}
	})

	t.Run("rejects invalid prefix", func(t *testing.T) {
		_, err := GenerateID("invalid")
		assert.Error(t, err)
	})
}

func TestGenerateChildID(t *testing.T) {
	t.Run("generates first child ID", func(t *testing.T) {
		childID := GenerateChildID("inv-ex4j", 1)
		assert.Equal(t, "inv-ex4j.1", childID)
	})

	t.Run("generates grandchild ID", func(t *testing.T) {
		childID := GenerateChildID("inv-ex4j.1", 1)
		assert.Equal(t, "inv-ex4j.1.1", childID)
	})
}

func TestValidateID(t *testing.T) {
	validIDs := []string{
		"ab-1234",
		"inv-ex4j",
		"abcd-0000",
		"inv-ex4j.1",
		"inv-ex4j.1.2",
		"inv-ex4j.1.2.3",
	}
	for _, id := range validIDs {
		t.Run("valid: "+id, func(t *testing.T) {
			assert.NoError(t, ValidateID(id))
		})
	}

	invalidIDs := []string{
		"",
		"inv",
		"inv-",
		"inv-ex",      // too short random part
		"inv-ex4j5",   // too long random part
		"INV-ex4j",    // uppercase prefix
		"inv-EX4J",    // uppercase random
		"a-1234",      // prefix too short
		"abcde-1234",  // prefix too long
		"inv-ex4j.",   // trailing dot
		"inv-ex4j.a",  // non-numeric sequence
		".inv-ex4j",   // leading dot
	}
	for _, id := range invalidIDs {
		t.Run("invalid: "+id, func(t *testing.T) {
			assert.Error(t, ValidateID(id))
		})
	}
}

func TestParseID(t *testing.T) {
	t.Run("parses root ID", func(t *testing.T) {
		prefix, base, seq, err := ParseID("inv-ex4j")
		require.NoError(t, err)
		assert.Equal(t, "inv-", prefix)
		assert.Equal(t, "ex4j", base)
		assert.Empty(t, seq)
	})

	t.Run("parses child ID", func(t *testing.T) {
		prefix, base, seq, err := ParseID("inv-ex4j.1")
		require.NoError(t, err)
		assert.Equal(t, "inv-", prefix)
		assert.Equal(t, "ex4j", base)
		assert.Equal(t, []int{1}, seq)
	})

	t.Run("parses deep hierarchy ID", func(t *testing.T) {
		prefix, base, seq, err := ParseID("ab-1234.1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "ab-", prefix)
		assert.Equal(t, "1234", base)
		assert.Equal(t, []int{1, 2, 3}, seq)
	})
}

func TestGetParentID(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"inv-ex4j", ""},
		{"inv-ex4j.1", "inv-ex4j"},
		{"inv-ex4j.1.2", "inv-ex4j.1"},
		{"inv-ex4j.1.2.3", "inv-ex4j.1.2"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetParentID(tt.id))
		})
	}
}

func TestGetRootID(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"inv-ex4j", "inv-ex4j"},
		{"inv-ex4j.1", "inv-ex4j"},
		{"inv-ex4j.1.2", "inv-ex4j"},
		{"inv-ex4j.1.2.3", "inv-ex4j"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetRootID(tt.id))
		})
	}
}

func TestIsChildOf(t *testing.T) {
	assert.True(t, IsChildOf("inv-ex4j.1", "inv-ex4j"))
	assert.True(t, IsChildOf("inv-ex4j.1.1", "inv-ex4j.1"))
	assert.False(t, IsChildOf("inv-ex4j", "inv-ex4j"))
	assert.False(t, IsChildOf("inv-ex4j.1.1", "inv-ex4j")) // grandchild, not direct child
}

func TestIsDescendantOf(t *testing.T) {
	assert.True(t, IsDescendantOf("inv-ex4j.1", "inv-ex4j"))
	assert.True(t, IsDescendantOf("inv-ex4j.1.1", "inv-ex4j"))
	assert.True(t, IsDescendantOf("inv-ex4j.1.1", "inv-ex4j.1"))
	assert.False(t, IsDescendantOf("inv-ex4j", "inv-ex4j"))
	assert.False(t, IsDescendantOf("inv-ex4j", "inv-ex4j.1"))
}

func TestGetDepth(t *testing.T) {
	tests := []struct {
		id       string
		expected int
	}{
		{"inv-ex4j", 0},
		{"inv-ex4j.1", 1},
		{"inv-ex4j.1.2", 2},
		{"inv-ex4j.1.2.3", 3},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetDepth(tt.id))
		})
	}
}

func TestGetChildSequence(t *testing.T) {
	tests := []struct {
		id       string
		expected int
	}{
		{"inv-ex4j", 0},      // root ID has no sequence
		{"inv-ex4j.1", 1},    // first child
		{"inv-ex4j.3", 3},    // third child
		{"inv-ex4j.1.5", 5},  // grandchild with seq 5
		{"inv-ex4j.1.10", 10}, // double-digit sequence
		{"inv-ex4j.99", 99},  // large sequence
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetChildSequence(tt.id))
		})
	}
}

func TestGenerateChildID_EdgeCases(t *testing.T) {
	t.Run("generates child with zero sequence", func(t *testing.T) {
		childID := GenerateChildID("inv-ex4j", 0)
		assert.Equal(t, "inv-ex4j.0", childID)
	})

	t.Run("generates child with large sequence", func(t *testing.T) {
		childID := GenerateChildID("inv-ex4j", 999)
		assert.Equal(t, "inv-ex4j.999", childID)
	})

	t.Run("generates deeply nested child", func(t *testing.T) {
		childID := GenerateChildID("inv-ex4j.1.2.3.4.5", 6)
		assert.Equal(t, "inv-ex4j.1.2.3.4.5.6", childID)
	})
}

func TestValidateID_EdgeCases(t *testing.T) {
	// Additional edge cases for ID validation
	invalidEdgeCases := []string{
		"inv--ex4j",    // double dash
		"inv-ex4j..",   // double dot
		"inv-ex4j.-1",  // negative sequence
		"inv-ex4j.1a",  // alphanumeric sequence
		"inv-ex4j.1.a", // alphanumeric in nested sequence
		"Inv-ex4j",     // capitalized prefix start
		"iNv-ex4j",     // capitalized prefix middle
	}
	for _, id := range invalidEdgeCases {
		t.Run("invalid: "+id, func(t *testing.T) {
			assert.Error(t, ValidateID(id))
		})
	}
}

func TestParseID_EdgeCases(t *testing.T) {
	t.Run("parses 2-char prefix", func(t *testing.T) {
		prefix, base, seq, err := ParseID("ab-1234")
		require.NoError(t, err)
		assert.Equal(t, "ab-", prefix)
		assert.Equal(t, "1234", base)
		assert.Empty(t, seq)
	})

	t.Run("parses 4-char prefix", func(t *testing.T) {
		prefix, base, seq, err := ParseID("abcd-5678")
		require.NoError(t, err)
		assert.Equal(t, "abcd-", prefix)
		assert.Equal(t, "5678", base)
		assert.Empty(t, seq)
	})

	t.Run("parses all-numeric base", func(t *testing.T) {
		prefix, base, seq, err := ParseID("ab-0000")
		require.NoError(t, err)
		assert.Equal(t, "ab-", prefix)
		assert.Equal(t, "0000", base)
		assert.Empty(t, seq)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		_, _, _, err := ParseID("invalid")
		assert.ErrorIs(t, err, ErrInvalidID)
	})

	t.Run("parses very deep hierarchy", func(t *testing.T) {
		prefix, base, seq, err := ParseID("ab-1234.1.2.3.4.5.6.7.8.9.10")
		require.NoError(t, err)
		assert.Equal(t, "ab-", prefix)
		assert.Equal(t, "1234", base)
		assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, seq)
	})
}

func TestIsChildOf_EdgeCases(t *testing.T) {
	t.Run("empty parent returns false", func(t *testing.T) {
		assert.False(t, IsChildOf("inv-ex4j.1", ""))
	})

	t.Run("same ID returns false", func(t *testing.T) {
		assert.False(t, IsChildOf("inv-ex4j.1", "inv-ex4j.1"))
	})
}

func TestIsDescendantOf_EdgeCases(t *testing.T) {
	t.Run("different prefix IDs", func(t *testing.T) {
		// Different prefixes should not be considered descendants
		assert.False(t, IsDescendantOf("abc-1234.1", "def-1234"))
	})

	t.Run("prefix substring match should not match", func(t *testing.T) {
		// "inv-ex4j.11" should not be descendant of "inv-ex4j.1"
		// because we require the prefix + "." pattern
		assert.False(t, IsDescendantOf("inv-ex4j.11", "inv-ex4j.1"))
	})

	t.Run("deeply nested is descendant of root", func(t *testing.T) {
		assert.True(t, IsDescendantOf("inv-ex4j.1.2.3.4.5", "inv-ex4j"))
	})
}

func TestGenerateID_MultiplePrefixes(t *testing.T) {
	prefixes := []string{"ab-", "abc-", "abcd-"}
	for _, prefix := range prefixes {
		t.Run("prefix: "+prefix, func(t *testing.T) {
			id, err := GenerateID(prefix)
			require.NoError(t, err)
			assert.True(t, len(id) == len(prefix)+4, "ID should be prefix length + 4 chars")
			assert.NoError(t, ValidateID(id))
		})
	}
}
