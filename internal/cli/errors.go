// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// Error codes for structured error responses
const (
	ErrCodeRecordNotFound  = "RECORD_NOT_FOUND"
	ErrCodeStashNotFound   = "STASH_NOT_FOUND"
	ErrCodeColumnNotFound  = "COLUMN_NOT_FOUND"
	ErrCodeValidation      = "VALIDATION_ERROR"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeReferenceError  = "REFERENCE_ERROR"
	ErrCodeRecordDeleted   = "RECORD_DELETED"
	ErrCodeNoStashDir      = "NO_STASH_DIR"
	ErrCodeInvalidSQL      = "INVALID_SQL"
	ErrCodePermissionError = "PERMISSION_ERROR"
)

// JSONError represents a structured error response for --json output
type JSONError struct {
	Error   bool                   `json:"error"`
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ExitWithError outputs an error message and exits.
// If --json flag is set, outputs structured JSON error to stdout.
// Otherwise outputs plain text to stderr.
func ExitWithError(code int, errCode, message string, details map[string]interface{}) {
	if GetJSONOutput() {
		errResp := JSONError{
			Error:   true,
			Code:    errCode,
			Message: message,
			Details: details,
		}
		data, _ := json.Marshal(errResp)
		fmt.Println(string(data))
	} else {
		fmt.Fprintln(os.Stderr, "Error:", message)
	}
	Exit(code)
}

// ExitRecordNotFound outputs a record not found error
func ExitRecordNotFound(recordID string) {
	ExitWithError(1, ErrCodeRecordNotFound,
		fmt.Sprintf("record '%s' not found", recordID),
		map[string]interface{}{"record_id": recordID})
}

// ExitStashNotFound outputs a stash not found error
func ExitStashNotFound(stashName string) {
	ExitWithError(1, ErrCodeStashNotFound,
		fmt.Sprintf("stash '%s' not found", stashName),
		map[string]interface{}{"stash": stashName})
}

// ExitColumnNotFound outputs a column not found error
func ExitColumnNotFound(columnName string) {
	ExitWithError(1, ErrCodeColumnNotFound,
		fmt.Sprintf("column '%s' not found", columnName),
		map[string]interface{}{"column": columnName})
}

// ExitValidationError outputs a validation error
func ExitValidationError(message string, details map[string]interface{}) {
	ExitWithError(2, ErrCodeValidation, message, details)
}

// ExitRecordDeleted outputs an error for attempting to modify a deleted record
func ExitRecordDeleted(recordID string) {
	ExitWithError(3, ErrCodeRecordDeleted,
		fmt.Sprintf("record '%s' is deleted (use 'stash restore' first)", recordID),
		map[string]interface{}{"record_id": recordID})
}

// ExitReferenceError outputs a reference error (e.g., invalid parent)
func ExitReferenceError(message string, details map[string]interface{}) {
	ExitWithError(4, ErrCodeReferenceError, message, details)
}

// ExitNoStashDir outputs an error when no .stash directory is found
func ExitNoStashDir() {
	ExitWithError(1, ErrCodeNoStashDir,
		"no .stash directory found",
		nil)
}

// ExitInvalidSQL outputs an error for invalid SQL
func ExitInvalidSQL(message string, query string) {
	ExitWithError(2, ErrCodeInvalidSQL, message,
		map[string]interface{}{"query": query})
}
