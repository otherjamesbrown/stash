package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateColumnName(t *testing.T) {
	validNames := []string{
		"Name",
		"name",
		"Price",
		"my_column",
		"Column123",
		"A",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 64 chars
	}
	for _, name := range validNames {
		t.Run("valid: "+name, func(t *testing.T) {
			assert.NoError(t, ValidateColumnName(name))
		})
	}

	invalidNames := []struct {
		name string
		err  error
	}{
		{"_id", ErrReservedColumn},
		{"_hash", ErrReservedColumn},
		{"_created_at", ErrReservedColumn},
		{"123name", ErrInvalidColumn},     // starts with number
		{"my-column", ErrInvalidColumn},   // contains hyphen
		{"my column", ErrInvalidColumn},   // contains space
		{"", ErrInvalidColumn},            // empty
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ErrInvalidColumn}, // 65 chars
	}
	for _, tt := range invalidNames {
		t.Run("invalid: "+tt.name, func(t *testing.T) {
			err := ValidateColumnName(tt.name)
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestIsReservedColumn(t *testing.T) {
	reserved := []string{
		"_id", "_hash", "_parent", "_created_at", "_created_by",
		"_updated_at", "_updated_by", "_branch", "_deleted_at",
		"_deleted_by", "_op",
	}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			assert.True(t, IsReservedColumn(name))
		})
	}

	notReserved := []string{"Name", "Price", "id", "hash", "name"}
	for _, name := range notReserved {
		t.Run(name, func(t *testing.T) {
			assert.False(t, IsReservedColumn(name))
		})
	}
}

func TestColumnList(t *testing.T) {
	now := time.Now()
	columns := ColumnList{
		{Name: "Name", Desc: "Item name", Added: now, AddedBy: "alice"},
		{Name: "Price", Desc: "Price in USD", Added: now, AddedBy: "alice"},
		{Name: "Category", Desc: "", Added: now, AddedBy: "bob"},
	}

	t.Run("Find exact match", func(t *testing.T) {
		col := columns.Find("Name")
		assert.NotNil(t, col)
		assert.Equal(t, "Name", col.Name)
	})

	t.Run("Find case-insensitive", func(t *testing.T) {
		col := columns.Find("name")
		assert.NotNil(t, col)
		assert.Equal(t, "Name", col.Name)

		col = columns.Find("NAME")
		assert.NotNil(t, col)
		assert.Equal(t, "Name", col.Name)
	})

	t.Run("Find returns nil for non-existent", func(t *testing.T) {
		col := columns.Find("NonExistent")
		assert.Nil(t, col)
	})

	t.Run("Exists", func(t *testing.T) {
		assert.True(t, columns.Exists("Name"))
		assert.True(t, columns.Exists("name"))
		assert.False(t, columns.Exists("NonExistent"))
	})

	t.Run("Index", func(t *testing.T) {
		assert.Equal(t, 0, columns.Index("Name"))
		assert.Equal(t, 1, columns.Index("price"))
		assert.Equal(t, 2, columns.Index("CATEGORY"))
		assert.Equal(t, -1, columns.Index("NonExistent"))
	})

	t.Run("Names", func(t *testing.T) {
		names := columns.Names()
		assert.Equal(t, []string{"Name", "Price", "Category"}, names)
	})

	t.Run("First", func(t *testing.T) {
		first := columns.First()
		assert.NotNil(t, first)
		assert.Equal(t, "Name", first.Name)
	})

	t.Run("First on empty list", func(t *testing.T) {
		empty := ColumnList{}
		assert.Nil(t, empty.First())
	})
}

func TestValidateColumnName_EdgeCases(t *testing.T) {
	t.Run("valid boundary cases", func(t *testing.T) {
		// Single character
		assert.NoError(t, ValidateColumnName("a"))
		assert.NoError(t, ValidateColumnName("A"))
		assert.NoError(t, ValidateColumnName("z"))
		assert.NoError(t, ValidateColumnName("Z"))
	})

	t.Run("allows numbers after first char", func(t *testing.T) {
		assert.NoError(t, ValidateColumnName("a123"))
		assert.NoError(t, ValidateColumnName("Column99"))
	})

	t.Run("allows underscores", func(t *testing.T) {
		assert.NoError(t, ValidateColumnName("my_column"))
		assert.NoError(t, ValidateColumnName("a_"))
		assert.NoError(t, ValidateColumnName("a___b"))
	})

	t.Run("rejects starting with underscore (not reserved)", func(t *testing.T) {
		// _column is not in reserved list but starts with underscore,
		// so it fails the regex validation
		err := ValidateColumnName("_column")
		assert.ErrorIs(t, err, ErrInvalidColumn)
	})

	t.Run("rejects hyphens", func(t *testing.T) {
		err := ValidateColumnName("my-column")
		assert.ErrorIs(t, err, ErrInvalidColumn)
	})

	t.Run("rejects spaces", func(t *testing.T) {
		err := ValidateColumnName("my column")
		assert.ErrorIs(t, err, ErrInvalidColumn)
	})

	t.Run("rejects dots", func(t *testing.T) {
		err := ValidateColumnName("my.column")
		assert.ErrorIs(t, err, ErrInvalidColumn)
	})

	t.Run("max length validation", func(t *testing.T) {
		// 64 characters should be valid
		maxLen := "a"
		for i := 0; i < 63; i++ {
			maxLen += "b"
		}
		assert.NoError(t, ValidateColumnName(maxLen))

		// 65 characters should be invalid
		tooLong := maxLen + "c"
		assert.ErrorIs(t, ValidateColumnName(tooLong), ErrInvalidColumn)
	})
}

func TestIsReservedColumn_EdgeCases(t *testing.T) {
	t.Run("case insensitive checks", func(t *testing.T) {
		assert.True(t, IsReservedColumn("_ID"))
		assert.True(t, IsReservedColumn("_Id"))
		assert.True(t, IsReservedColumn("_HASH"))
		assert.True(t, IsReservedColumn("_Created_At"))
	})

	t.Run("similar but not reserved names", func(t *testing.T) {
		assert.False(t, IsReservedColumn("id"))      // no underscore
		assert.False(t, IsReservedColumn("hash"))    // no underscore
		assert.False(t, IsReservedColumn("_custom")) // not in reserved list
	})
}

func TestColumnList_EmptyList(t *testing.T) {
	empty := ColumnList{}

	t.Run("Find on empty list", func(t *testing.T) {
		assert.Nil(t, empty.Find("anything"))
	})

	t.Run("Exists on empty list", func(t *testing.T) {
		assert.False(t, empty.Exists("anything"))
	})

	t.Run("Index on empty list", func(t *testing.T) {
		assert.Equal(t, -1, empty.Index("anything"))
	})

	t.Run("Names on empty list", func(t *testing.T) {
		names := empty.Names()
		assert.NotNil(t, names)
		assert.Empty(t, names)
	})

	t.Run("First on empty list", func(t *testing.T) {
		assert.Nil(t, empty.First())
	})
}

func TestColumnList_SingleColumn(t *testing.T) {
	single := ColumnList{
		{Name: "OnlyColumn", Desc: "The only one"},
	}

	t.Run("Find single column", func(t *testing.T) {
		col := single.Find("OnlyColumn")
		assert.NotNil(t, col)
		assert.Equal(t, "OnlyColumn", col.Name)
	})

	t.Run("Index of single column", func(t *testing.T) {
		assert.Equal(t, 0, single.Index("OnlyColumn"))
	})

	t.Run("Names with single column", func(t *testing.T) {
		names := single.Names()
		assert.Equal(t, []string{"OnlyColumn"}, names)
	})

	t.Run("First returns the single column", func(t *testing.T) {
		col := single.First()
		assert.NotNil(t, col)
		assert.Equal(t, "OnlyColumn", col.Name)
	})
}

func TestColumnList_Modification(t *testing.T) {
	t.Run("modifying found column modifies list", func(t *testing.T) {
		columns := ColumnList{
			{Name: "Test", Desc: "Original"},
		}

		// Find returns a pointer to the column in the list
		col := columns.Find("Test")
		col.Desc = "Modified"

		// The original list should be modified
		assert.Equal(t, "Modified", columns[0].Desc)
	})
}
