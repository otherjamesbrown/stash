// Package model provides core data types for stash.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

// Attachment represents a file attached to a record.
type Attachment struct {
	Name       string    `json:"name"`        // Original filename
	Size       int64     `json:"size"`        // File size in bytes
	Hash       string    `json:"hash"`        // SHA-256 hash of file content
	AttachedAt time.Time `json:"attached_at"` // When the file was attached
	AttachedBy string    `json:"attached_by"` // Who attached the file
}

// CalculateFileHash computes SHA-256 hash of a file.
// Returns the hex-encoded hash string.
func CalculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Error types for attachment operations
var (
	ErrFileNotFound       = newError("file not found")
	ErrAttachmentNotFound = newError("attachment not found")
	ErrAttachmentExists   = newError("attachment already exists")
)

func newError(msg string) error {
	return &stashError{msg: msg}
}

type stashError struct {
	msg string
}

func (e *stashError) Error() string {
	return e.msg
}
