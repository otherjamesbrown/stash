package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePrefix(t *testing.T) {
	validPrefixes := []string{
		"ab-",   // 3 chars (min)
		"inv-",  // 4 chars
		"abcd-", // 5 chars (max)
	}
	for _, prefix := range validPrefixes {
		t.Run("valid: "+prefix, func(t *testing.T) {
			assert.NoError(t, ValidatePrefix(prefix))
		})
	}

	invalidPrefixes := []string{
		"a-",      // too short (2 chars)
		"abcde-",  // too long (6 chars)
		"inv",     // no dash
		"INV-",    // uppercase
		"in1-",    // contains number
		"i-v-",    // multiple dashes
		"-inv",    // dash at start
		"",        // empty
	}
	for _, prefix := range invalidPrefixes {
		t.Run("invalid: "+prefix, func(t *testing.T) {
			err := ValidatePrefix(prefix)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidPrefix)
		})
	}
}

func TestValidateStashName(t *testing.T) {
	validNames := []string{
		"inventory",
		"Inventory",
		"my-stash",
		"my_stash",
		"stash123",
		"a",
	}
	for _, name := range validNames {
		t.Run("valid: "+name, func(t *testing.T) {
			assert.NoError(t, ValidateStashName(name))
		})
	}

	invalidNames := []string{
		"",           // empty
		"123stash",   // starts with number
		"-stash",     // starts with hyphen
		"my stash",   // contains space
		"my@stash",   // contains special char
	}
	for _, name := range invalidNames {
		t.Run("invalid: "+name, func(t *testing.T) {
			assert.Error(t, ValidateStashName(name))
		})
	}
}

func TestStashAddColumn(t *testing.T) {
	now := time.Now()

	t.Run("adds new column", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		col := Column{Name: "Name", Desc: "Item name", Added: now, AddedBy: "alice"}

		err := s.AddColumn(col)
		require.NoError(t, err)
		assert.Len(t, s.Columns, 1)
		assert.Equal(t, "Name", s.Columns[0].Name)
	})

	t.Run("rejects duplicate column (exact case)", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		s.AddColumn(Column{Name: "Name", Added: now, AddedBy: "alice"})

		err := s.AddColumn(Column{Name: "Name", Added: now, AddedBy: "bob"})
		assert.ErrorIs(t, err, ErrColumnExists)
	})

	t.Run("rejects duplicate column (case-insensitive)", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		s.AddColumn(Column{Name: "Name", Added: now, AddedBy: "alice"})

		err := s.AddColumn(Column{Name: "name", Added: now, AddedBy: "bob"})
		assert.ErrorIs(t, err, ErrColumnExists)
		assert.Contains(t, err.Error(), "Name") // Should show original case
	})

	t.Run("rejects reserved column name", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}

		err := s.AddColumn(Column{Name: "_id", Added: now, AddedBy: "alice"})
		assert.ErrorIs(t, err, ErrReservedColumn)
	})

	t.Run("rejects invalid column name", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}

		err := s.AddColumn(Column{Name: "123invalid", Added: now, AddedBy: "alice"})
		assert.ErrorIs(t, err, ErrInvalidColumn)
	})
}

func TestStashGetColumn(t *testing.T) {
	now := time.Now()
	s := &Stash{
		Name:   "test",
		Prefix: "ts-",
		Columns: ColumnList{
			{Name: "Name", Desc: "Item name", Added: now, AddedBy: "alice"},
			{Name: "Price", Desc: "Price in USD", Added: now, AddedBy: "alice"},
		},
	}

	t.Run("finds existing column", func(t *testing.T) {
		col, err := s.GetColumn("Name")
		require.NoError(t, err)
		assert.Equal(t, "Name", col.Name)
	})

	t.Run("finds column case-insensitive", func(t *testing.T) {
		col, err := s.GetColumn("name")
		require.NoError(t, err)
		assert.Equal(t, "Name", col.Name)
	})

	t.Run("returns error for non-existent", func(t *testing.T) {
		_, err := s.GetColumn("NonExistent")
		assert.ErrorIs(t, err, ErrColumnNotFound)
	})
}

func TestStashHasColumns(t *testing.T) {
	t.Run("returns false when empty", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		assert.False(t, s.HasColumns())
	})

	t.Run("returns true when has columns", func(t *testing.T) {
		s := &Stash{
			Name:    "test",
			Prefix:  "ts-",
			Columns: ColumnList{{Name: "Name"}},
		}
		assert.True(t, s.HasColumns())
	})
}

func TestStashPrimaryColumn(t *testing.T) {
	t.Run("returns first column", func(t *testing.T) {
		s := &Stash{
			Name:   "test",
			Prefix: "ts-",
			Columns: ColumnList{
				{Name: "Name"},
				{Name: "Price"},
			},
		}
		col := s.PrimaryColumn()
		assert.NotNil(t, col)
		assert.Equal(t, "Name", col.Name)
	})

	t.Run("returns nil when no columns", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		assert.Nil(t, s.PrimaryColumn())
	})
}

func TestValidatePrefix_EdgeCases(t *testing.T) {
	// Test specific error messages
	t.Run("error message for too short prefix", func(t *testing.T) {
		err := ValidatePrefix("a-")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
		assert.Contains(t, err.Error(), "3-5 characters")
	})

	t.Run("error message for too long prefix", func(t *testing.T) {
		err := ValidatePrefix("abcdef-")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
		assert.Contains(t, err.Error(), "3-5 characters")
	})

	t.Run("error message for missing dash", func(t *testing.T) {
		err := ValidatePrefix("abc")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
		assert.Contains(t, err.Error(), "must end with dash")
	})

	t.Run("error message for invalid characters", func(t *testing.T) {
		err := ValidatePrefix("AB-")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
		assert.Contains(t, err.Error(), "lowercase letters")
	})

	t.Run("error for numbers in prefix", func(t *testing.T) {
		err := ValidatePrefix("a1-")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
	})

	t.Run("error for special characters in prefix", func(t *testing.T) {
		err := ValidatePrefix("a@-")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
	})

	t.Run("error for space in prefix", func(t *testing.T) {
		err := ValidatePrefix("a -")
		assert.ErrorIs(t, err, ErrInvalidPrefix)
	})
}

func TestValidateStashName_EdgeCases(t *testing.T) {
	// Test boundary conditions
	t.Run("valid single character name", func(t *testing.T) {
		assert.NoError(t, ValidateStashName("a"))
	})

	t.Run("valid 64 character name (max)", func(t *testing.T) {
		longName := ""
		for i := 0; i < 64; i++ {
			longName += "a"
		}
		assert.NoError(t, ValidateStashName(longName))
	})

	t.Run("invalid 65 character name (over max)", func(t *testing.T) {
		longName := ""
		for i := 0; i < 65; i++ {
			longName += "a"
		}
		assert.Error(t, ValidateStashName(longName))
	})

	t.Run("valid name with all allowed characters", func(t *testing.T) {
		assert.NoError(t, ValidateStashName("My_Stash-Name123"))
	})

	t.Run("invalid name starting with underscore", func(t *testing.T) {
		assert.Error(t, ValidateStashName("_stash"))
	})

	t.Run("invalid name with dot", func(t *testing.T) {
		assert.Error(t, ValidateStashName("my.stash"))
	})

	t.Run("invalid name with slash", func(t *testing.T) {
		assert.Error(t, ValidateStashName("my/stash"))
	})

	t.Run("invalid name with backslash", func(t *testing.T) {
		assert.Error(t, ValidateStashName("my\\stash"))
	})
}

func TestStashAddColumn_EdgeCases(t *testing.T) {
	now := time.Now()

	t.Run("rejects all reserved column names", func(t *testing.T) {
		reserved := []string{
			"_id", "_hash", "_parent", "_created_at", "_created_by",
			"_updated_at", "_updated_by", "_branch", "_deleted_at",
			"_deleted_by", "_op",
		}
		for _, name := range reserved {
			s := &Stash{Name: "test", Prefix: "ts-"}
			err := s.AddColumn(Column{Name: name, Added: now, AddedBy: "alice"})
			assert.ErrorIs(t, err, ErrReservedColumn, "expected ErrReservedColumn for %s", name)
		}
	})

	t.Run("rejects reserved column names case-insensitive", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		err := s.AddColumn(Column{Name: "_ID", Added: now, AddedBy: "alice"})
		assert.ErrorIs(t, err, ErrReservedColumn)
	})

	t.Run("adds multiple unique columns", func(t *testing.T) {
		s := &Stash{Name: "test", Prefix: "ts-"}
		err := s.AddColumn(Column{Name: "Name", Added: now, AddedBy: "alice"})
		require.NoError(t, err)
		err = s.AddColumn(Column{Name: "Price", Added: now, AddedBy: "alice"})
		require.NoError(t, err)
		err = s.AddColumn(Column{Name: "Quantity", Added: now, AddedBy: "alice"})
		require.NoError(t, err)
		assert.Len(t, s.Columns, 3)
	})
}

func TestStashGetColumn_EdgeCases(t *testing.T) {
	now := time.Now()
	s := &Stash{
		Name:   "test",
		Prefix: "ts-",
		Columns: ColumnList{
			{Name: "CamelCaseName", Desc: "Test", Added: now, AddedBy: "alice"},
		},
	}

	t.Run("finds column with mixed case query", func(t *testing.T) {
		col, err := s.GetColumn("camelcasename")
		require.NoError(t, err)
		assert.Equal(t, "CamelCaseName", col.Name)
	})

	t.Run("returns original case in result", func(t *testing.T) {
		col, err := s.GetColumn("CAMELCASENAME")
		require.NoError(t, err)
		assert.Equal(t, "CamelCaseName", col.Name)
	})
}
