package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Operation types for JSONL records
const (
	OpCreate  = "create"
	OpUpdate  = "update"
	OpDelete  = "delete"
	OpRestore = "restore"
)

// Record represents a single record in a stash.
type Record struct {
	ID        string     `json:"_id"`
	Hash      string     `json:"_hash"`
	ParentID  string     `json:"_parent,omitempty"`
	CreatedAt time.Time  `json:"_created_at"`
	CreatedBy string     `json:"_created_by"`
	UpdatedAt time.Time  `json:"_updated_at"`
	UpdatedBy string     `json:"_updated_by"`
	Branch    string     `json:"_branch,omitempty"`
	DeletedAt *time.Time `json:"_deleted_at,omitempty"`
	DeletedBy string     `json:"_deleted_by,omitempty"`
	Operation string     `json:"_op"`
	Fields    map[string]interface{}
}

// IsDeleted returns true if the record has been soft-deleted.
func (r *Record) IsDeleted() bool {
	return r.DeletedAt != nil
}

// CalculateHash computes the SHA-256 hash of the record's user fields.
// The hash is deterministic: same fields produce the same hash.
// Returns the first 12 characters of the hex-encoded hash.
func (r *Record) CalculateHash() string {
	return CalculateHash(r.Fields)
}

// CalculateHash computes a deterministic hash from a map of fields.
// Only non-system fields (those not starting with "_") are included.
// Returns the first 12 characters of the hex-encoded SHA-256 hash.
func CalculateHash(fields map[string]interface{}) string {
	// 1. Extract only user fields (exclude _ prefixed)
	userFields := make(map[string]interface{})
	for k, v := range fields {
		if !strings.HasPrefix(k, "_") {
			userFields[k] = v
		}
	}

	// 2. Sort keys for deterministic ordering
	keys := make([]string, 0, len(userFields))
	for k := range userFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 3. Build canonical representation
	var buf bytes.Buffer
	for _, k := range keys {
		v, _ := json.Marshal(userFields[k])
		buf.WriteString(k)
		buf.WriteString(":")
		buf.Write(v)
		buf.WriteString("\n")
	}

	// 4. SHA-256 hash, return first 12 chars
	hash := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(hash[:])[:12]
}

// MarshalJSON implements custom JSON marshaling that flattens Fields into the output.
func (r *Record) MarshalJSON() ([]byte, error) {
	// Create a map with all system fields
	m := make(map[string]interface{})
	m["_id"] = r.ID
	m["_hash"] = r.Hash
	m["_op"] = r.Operation
	m["_created_at"] = r.CreatedAt
	m["_created_by"] = r.CreatedBy
	m["_updated_at"] = r.UpdatedAt
	m["_updated_by"] = r.UpdatedBy

	if r.ParentID != "" {
		m["_parent"] = r.ParentID
	}
	if r.Branch != "" {
		m["_branch"] = r.Branch
	}
	if r.DeletedAt != nil {
		m["_deleted_at"] = r.DeletedAt
		m["_deleted_by"] = r.DeletedBy
	}

	// Merge user fields
	for k, v := range r.Fields {
		m[k] = v
	}

	return json.Marshal(m)
}

// UnmarshalJSON implements custom JSON unmarshaling that extracts Fields.
func (r *Record) UnmarshalJSON(data []byte) error {
	// First unmarshal into a generic map
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Extract system fields
	if v, ok := m["_id"].(string); ok {
		r.ID = v
	}
	if v, ok := m["_hash"].(string); ok {
		r.Hash = v
	}
	if v, ok := m["_parent"].(string); ok {
		r.ParentID = v
	}
	if v, ok := m["_op"].(string); ok {
		r.Operation = v
	}
	if v, ok := m["_created_by"].(string); ok {
		r.CreatedBy = v
	}
	if v, ok := m["_updated_by"].(string); ok {
		r.UpdatedBy = v
	}
	if v, ok := m["_branch"].(string); ok {
		r.Branch = v
	}
	if v, ok := m["_deleted_by"].(string); ok {
		r.DeletedBy = v
	}

	// Parse timestamps
	if v, ok := m["_created_at"].(string); ok {
		t, _ := time.Parse(time.RFC3339, v)
		r.CreatedAt = t
	}
	if v, ok := m["_updated_at"].(string); ok {
		t, _ := time.Parse(time.RFC3339, v)
		r.UpdatedAt = t
	}
	if v, ok := m["_deleted_at"]; ok && v != nil {
		if s, ok := v.(string); ok {
			t, _ := time.Parse(time.RFC3339, s)
			r.DeletedAt = &t
		}
	}

	// Extract user fields (everything not starting with "_")
	r.Fields = make(map[string]interface{})
	for k, v := range m {
		if !strings.HasPrefix(k, "_") {
			r.Fields[k] = v
		}
	}

	return nil
}

// GetField returns the value of a field, using case-insensitive matching.
func (r *Record) GetField(name string) (interface{}, bool) {
	// Try exact match first
	if v, ok := r.Fields[name]; ok {
		return v, true
	}
	// Try case-insensitive match
	nameLower := strings.ToLower(name)
	for k, v := range r.Fields {
		if strings.ToLower(k) == nameLower {
			return v, true
		}
	}
	return nil, false
}

// SetField sets a field value, using case-insensitive key matching.
// If the field exists (case-insensitive), updates it using the original case.
// If new, uses the provided case.
func (r *Record) SetField(name string, value interface{}) {
	if r.Fields == nil {
		r.Fields = make(map[string]interface{})
	}

	// Find existing key with case-insensitive match
	nameLower := strings.ToLower(name)
	for k := range r.Fields {
		if strings.ToLower(k) == nameLower {
			r.Fields[k] = value
			return
		}
	}

	// New field - use provided case
	r.Fields[name] = value
}
