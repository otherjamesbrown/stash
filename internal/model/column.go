package model

import (
	"regexp"
	"strings"
	"time"
)

// Reserved column names (system fields)
var reservedColumnNames = map[string]bool{
	"_id":         true,
	"_hash":       true,
	"_parent":     true,
	"_created_at": true,
	"_created_by": true,
	"_updated_at": true,
	"_updated_by": true,
	"_branch":     true,
	"_deleted_at": true,
	"_deleted_by": true,
	"_op":         true,
}

// Column name validation regex:
// - Must start with a letter
// - Can contain letters, numbers, underscores
// - Max 64 characters
var columnNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

// Column represents a user-defined column in a stash schema.
type Column struct {
	Name     string    `json:"name"`
	Desc     string    `json:"desc,omitempty"`
	Added    time.Time `json:"added"`
	AddedBy  string    `json:"added_by"`
	Validate string    `json:"validate,omitempty"` // Validation type: "email", "url", "number", "date"
	Enum     []string  `json:"enum,omitempty"`     // Allowed values for enum validation
	Required bool      `json:"required,omitempty"` // Whether field is required
}

// ValidateColumnName checks if a column name is valid.
// Returns nil if valid, or an appropriate error.
func ValidateColumnName(name string) error {
	// Check for reserved names (case-insensitive)
	if reservedColumnNames[strings.ToLower(name)] {
		return ErrReservedColumn
	}

	// Check format
	if !columnNameRegex.MatchString(name) {
		return ErrInvalidColumn
	}

	return nil
}

// IsReservedColumn returns true if the name is a reserved system column.
func IsReservedColumn(name string) bool {
	return reservedColumnNames[strings.ToLower(name)]
}

// ColumnList provides case-insensitive column operations.
type ColumnList []Column

// Find returns the column with the given name (case-insensitive).
// Returns nil if not found.
func (cl ColumnList) Find(name string) *Column {
	nameLower := strings.ToLower(name)
	for i := range cl {
		if strings.ToLower(cl[i].Name) == nameLower {
			return &cl[i]
		}
	}
	return nil
}

// Exists returns true if a column with the given name exists (case-insensitive).
func (cl ColumnList) Exists(name string) bool {
	return cl.Find(name) != nil
}

// Index returns the index of the column with the given name (case-insensitive).
// Returns -1 if not found.
func (cl ColumnList) Index(name string) int {
	nameLower := strings.ToLower(name)
	for i := range cl {
		if strings.ToLower(cl[i].Name) == nameLower {
			return i
		}
	}
	return -1
}

// Names returns all column names.
func (cl ColumnList) Names() []string {
	names := make([]string, len(cl))
	for i, c := range cl {
		names[i] = c.Name
	}
	return names
}

// First returns the first column, or nil if empty.
func (cl ColumnList) First() *Column {
	if len(cl) == 0 {
		return nil
	}
	return &cl[0]
}
