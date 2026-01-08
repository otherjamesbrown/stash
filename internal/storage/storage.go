// Package storage provides persistent storage for stash records.
package storage

import (
	"github.com/user/stash/internal/model"
)

// WhereCondition represents a single filter condition.
type WhereCondition struct {
	Field    string // Field name (column)
	Operator string // =, !=, <, >, <=, >=, LIKE
	Value    string // Value to compare against
}

// ListOptions configures record listing behavior.
type ListOptions struct {
	// IncludeDeleted includes soft-deleted records in the result.
	IncludeDeleted bool
	// DeletedOnly shows only deleted records (when combined with IncludeDeleted).
	DeletedOnly bool
	// ParentID filters records by parent (empty = root records only, "*" = all).
	ParentID string
	// Limit restricts the number of results (0 = no limit).
	Limit int
	// Offset skips the first N results.
	Offset int
	// OrderBy specifies the sort field.
	OrderBy string
	// Descending reverses the sort order.
	Descending bool
	// Where specifies filter conditions (ANDed together).
	Where []WhereCondition
	// Search specifies a full-text search term across all fields.
	Search string
	// Columns specifies which columns to return (empty = all).
	Columns []string
}

// Storage defines the interface for stash persistence.
type Storage interface {
	// Stash management
	CreateStash(name, prefix string, config *model.Stash) error
	DropStash(name string) error
	GetStash(name string) (*model.Stash, error)
	ListStashes() ([]*model.Stash, error)

	// Column management
	AddColumn(stashName string, col model.Column) error

	// Record operations
	CreateRecord(stashName string, record *model.Record) error
	UpdateRecord(stashName string, record *model.Record) error
	DeleteRecord(stashName string, id string, actor string) error
	RestoreRecord(stashName string, id string, actor string) error
	GetRecord(stashName string, id string) (*model.Record, error)
	ListRecords(stashName string, opts ListOptions) ([]*model.Record, error)

	// Child record operations
	GetChildren(stashName string, parentID string) ([]*model.Record, error)
	GetNextChildSeq(stashName string, parentID string) (int, error)

	// Sync operations
	RebuildCache(stashName string) error
	FlushToJSONL(stashName string) error

	// Close releases resources.
	Close() error
}
