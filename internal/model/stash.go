package model

import (
	"fmt"
	"regexp"
	"time"
)

// Prefix validation:
// - Must be 3-5 characters total
// - Must be 2-4 lowercase letters followed by a dash
// - Examples: ab-, inv-, abcd-
var prefixRegex = regexp.MustCompile(`^[a-z]{2,4}-$`)

// Stash name validation:
// - Must start with a letter
// - Can contain letters, numbers, hyphens, underscores
// - Max 64 characters
var stashNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// Stash represents a named collection of records with a shared prefix.
type Stash struct {
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Created   time.Time  `json:"created"`
	CreatedBy string     `json:"created_by"`
	Columns   ColumnList `json:"columns"`
}

// ValidatePrefix checks if a prefix is valid.
// Prefix must be 3-5 characters: 2-4 lowercase letters followed by a dash.
// Returns nil if valid, or an error with details.
func ValidatePrefix(prefix string) error {
	if len(prefix) < 3 || len(prefix) > 5 {
		return fmt.Errorf("%w: must be 3-5 characters (2-4 letters + dash), got %d", ErrInvalidPrefix, len(prefix))
	}

	if prefix[len(prefix)-1] != '-' {
		return fmt.Errorf("%w: must end with dash", ErrInvalidPrefix)
	}

	if !prefixRegex.MatchString(prefix) {
		return fmt.Errorf("%w: must be 2-4 lowercase letters followed by dash (e.g., inv-, ab-, abcd-)", ErrInvalidPrefix)
	}

	return nil
}

// ValidateStashName checks if a stash name is valid.
// Returns nil if valid, or an error with details.
func ValidateStashName(name string) error {
	if name == "" {
		return fmt.Errorf("stash name cannot be empty")
	}

	if !stashNameRegex.MatchString(name) {
		return fmt.Errorf("stash name must start with a letter and contain only letters, numbers, hyphens, and underscores")
	}

	return nil
}

// AddColumn adds a new column to the stash.
// Returns an error if the column already exists (case-insensitive).
func (s *Stash) AddColumn(col Column) error {
	if s.Columns.Exists(col.Name) {
		existing := s.Columns.Find(col.Name)
		return fmt.Errorf("%w: column '%s' already exists", ErrColumnExists, existing.Name)
	}

	if err := ValidateColumnName(col.Name); err != nil {
		return err
	}

	s.Columns = append(s.Columns, col)
	return nil
}

// GetColumn returns the column with the given name (case-insensitive).
func (s *Stash) GetColumn(name string) (*Column, error) {
	col := s.Columns.Find(name)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	return col, nil
}

// HasColumns returns true if the stash has at least one column.
func (s *Stash) HasColumns() bool {
	return len(s.Columns) > 0
}

// PrimaryColumn returns the first column (used for the primary value in add).
func (s *Stash) PrimaryColumn() *Column {
	return s.Columns.First()
}
