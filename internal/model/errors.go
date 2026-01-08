// Package model provides core data types for stash.
package model

import "errors"

// Error types for stash operations
var (
	ErrStashNotFound    = errors.New("stash not found")
	ErrStashExists      = errors.New("stash already exists")
	ErrRecordNotFound   = errors.New("record not found")
	ErrRecordDeleted    = errors.New("record is deleted")
	ErrColumnNotFound   = errors.New("column not found")
	ErrColumnExists     = errors.New("column already exists")
	ErrInvalidID        = errors.New("invalid record ID")
	ErrInvalidPrefix    = errors.New("invalid prefix")
	ErrParentNotFound   = errors.New("parent record not found")
	ErrDaemonNotRunning = errors.New("daemon not running")
	ErrSyncConflict     = errors.New("sync conflict detected")
	ErrHashMismatch     = errors.New("hash mismatch detected")
	ErrEmptyValue       = errors.New("empty value not allowed")
	ErrReservedColumn   = errors.New("reserved column name")
	ErrInvalidColumn    = errors.New("invalid column name")
	ErrHasChildren      = errors.New("record has children")
)
