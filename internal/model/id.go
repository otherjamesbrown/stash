package model

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Base36 character set (lowercase)
const base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"

// IDLength is the length of the random part of an ID
const IDLength = 4

// ID format regex for validation
// Matches: prefix-xxxx or prefix-xxxx.N or prefix-xxxx.N.M etc.
var idRegex = regexp.MustCompile(`^[a-z]{2,4}-[0-9a-z]{4}(\.\d+)*$`)

// GenerateID creates a new random ID with the given prefix.
// Format: <prefix><4-char-base36>
// Example: inv-ex4j
func GenerateID(prefix string) (string, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return "", err
	}

	random, err := randomBase36(IDLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	// Prefix already includes the dash
	return prefix + random, nil
}

// GenerateChildID creates a child ID from a parent ID.
// Format: <parent-id>.<next-seq>
// Example: inv-ex4j.1, inv-ex4j.2, inv-ex4j.1.1
func GenerateChildID(parentID string, nextSeq int) string {
	return fmt.Sprintf("%s.%d", parentID, nextSeq)
}

// ValidateID checks if an ID is valid.
func ValidateID(id string) error {
	if !idRegex.MatchString(id) {
		return ErrInvalidID
	}
	return nil
}

// ParseID extracts components from an ID.
// Returns prefix, base (random part), and sequence numbers.
// Example: "inv-ex4j.1.2" -> "inv-", "ex4j", [1, 2]
func ParseID(id string) (prefix, base string, seq []int, err error) {
	if err = ValidateID(id); err != nil {
		return
	}

	// Find the dash separating prefix from rest
	dashIdx := strings.Index(id, "-")
	if dashIdx == -1 {
		err = ErrInvalidID
		return
	}

	prefix = id[:dashIdx+1]

	// Get the rest after prefix
	rest := id[dashIdx+1:]

	// Split by dots
	parts := strings.Split(rest, ".")
	base = parts[0]

	// Parse sequence numbers
	for i := 1; i < len(parts); i++ {
		n, parseErr := strconv.Atoi(parts[i])
		if parseErr != nil {
			err = ErrInvalidID
			return
		}
		seq = append(seq, n)
	}

	return
}

// GetParentID returns the parent ID of a child ID.
// Returns empty string if the ID is a root ID (no parent).
// Example: "inv-ex4j.1.2" -> "inv-ex4j.1"
// Example: "inv-ex4j.1" -> "inv-ex4j"
// Example: "inv-ex4j" -> ""
func GetParentID(id string) string {
	lastDot := strings.LastIndex(id, ".")
	if lastDot == -1 {
		return "" // Root ID, no parent
	}
	return id[:lastDot]
}

// GetRootID returns the root ID from any ID in the hierarchy.
// Example: "inv-ex4j.1.2" -> "inv-ex4j"
func GetRootID(id string) string {
	firstDot := strings.Index(id, ".")
	if firstDot == -1 {
		return id // Already a root ID
	}
	return id[:firstDot]
}

// IsChildOf returns true if childID is a direct child of parentID.
func IsChildOf(childID, parentID string) bool {
	return GetParentID(childID) == parentID
}

// IsDescendantOf returns true if descendantID is a descendant of ancestorID.
func IsDescendantOf(descendantID, ancestorID string) bool {
	if descendantID == ancestorID {
		return false
	}
	return strings.HasPrefix(descendantID, ancestorID+".")
}

// GetDepth returns the depth of an ID in the hierarchy.
// Root IDs have depth 0.
func GetDepth(id string) int {
	return strings.Count(id, ".")
}

// GetChildSequence returns the sequence number of a child ID.
// Returns 0 for root IDs.
// Example: "inv-ex4j.3" -> 3
func GetChildSequence(id string) int {
	lastDot := strings.LastIndex(id, ".")
	if lastDot == -1 {
		return 0
	}
	seq, _ := strconv.Atoi(id[lastDot+1:])
	return seq
}

// randomBase36 generates a random base36 string of the given length.
func randomBase36(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(base36Chars)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = base36Chars[n.Int64()]
	}

	return string(result), nil
}
