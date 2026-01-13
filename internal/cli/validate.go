// Package cli provides the command-line interface for stash.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/stash/internal/context"
	"github.com/user/stash/internal/model"
	"github.com/user/stash/internal/storage"
)

// ValidationType defines the supported validation types
type ValidationType string

const (
	ValidationEmail  ValidationType = "email"
	ValidationURL    ValidationType = "url"
	ValidationNumber ValidationType = "number"
	ValidationDate   ValidationType = "date"
)

// ValidValidationTypes lists all valid validation type strings
var ValidValidationTypes = []string{
	string(ValidationEmail),
	string(ValidationURL),
	string(ValidationNumber),
	string(ValidationDate),
}

// Email validation regex (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// IsValidValidationType checks if a validation type string is valid
func IsValidValidationType(t string) bool {
	for _, valid := range ValidValidationTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// ValidationError represents a single validation error
type ValidationError struct {
	Column   string `json:"column"`
	Value    string `json:"value"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	RecordID string `json:"record_id,omitempty"`
}

// ValidationResult represents the result of validating a value against column constraints
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidateValue validates a single value against a column's constraints
func ValidateValue(col *model.Column, value interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Convert value to string for validation
	strValue := ""
	if value != nil {
		strValue = fmt.Sprintf("%v", value)
	}

	// Check required constraint
	if col.Required && (value == nil || strValue == "") {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Column:  col.Name,
			Value:   strValue,
			Rule:    "required",
			Message: fmt.Sprintf("column '%s' is required", col.Name),
		})
		return result // No need to check other constraints if required fails
	}

	// Skip other validations if value is empty (and not required)
	if strValue == "" {
		return result
	}

	// Check enum constraint
	if len(col.Enum) > 0 {
		found := false
		for _, allowed := range col.Enum {
			if strValue == allowed {
				found = true
				break
			}
		}
		if !found {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Column:  col.Name,
				Value:   strValue,
				Rule:    "enum",
				Message: fmt.Sprintf("value '%s' not in allowed values: %s", strValue, strings.Join(col.Enum, ", ")),
			})
		}
	}

	// Check validation type constraint
	if col.Validate != "" {
		var err error
		switch ValidationType(col.Validate) {
		case ValidationEmail:
			err = validateEmail(strValue)
		case ValidationURL:
			err = validateURL(strValue)
		case ValidationNumber:
			err = validateNumber(strValue)
		case ValidationDate:
			err = validateDate(strValue)
		}

		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Column:  col.Name,
				Value:   strValue,
				Rule:    col.Validate,
				Message: err.Error(),
			})
		}
	}

	return result
}

// validateEmail checks if a string is a valid email address
func validateEmail(value string) error {
	if !emailRegex.MatchString(value) {
		return fmt.Errorf("invalid email format: '%s'", value)
	}
	return nil
}

// validateURL checks if a string is a valid URL
func validateURL(value string) error {
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid URL format: '%s'", value)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL format (missing scheme or host): '%s'", value)
	}
	return nil
}

// validateNumber checks if a string is a valid number
func validateNumber(value string) error {
	// Try integer first
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return nil
	}
	// Try float
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return nil
	}
	return fmt.Errorf("invalid number format: '%s'", value)
}

// validateDate checks if a string is a valid ISO date
func validateDate(value string) error {
	// Try common date formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
		time.DateOnly,
	}

	for _, format := range formats {
		if _, err := time.Parse(format, value); err == nil {
			return nil
		}
	}
	return fmt.Errorf("invalid date format: '%s' (expected ISO format like 2006-01-02 or 2006-01-02T15:04:05Z)", value)
}

// ValidateRecord validates all fields in a record against column constraints
func ValidateRecord(stash *model.Stash, record *model.Record) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}

	for _, col := range stash.Columns {
		value := record.Fields[col.Name]
		colResult := ValidateValue(&col, value)
		if !colResult.Valid {
			result.Valid = false
			for i := range colResult.Errors {
				colResult.Errors[i].RecordID = record.ID
			}
			result.Errors = append(result.Errors, colResult.Errors...)
		}
	}

	return result
}

// ValidateFields validates a map of field values against stash columns
func ValidateFields(stash *model.Stash, fields map[string]interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Check fields that are being set
	for fieldName, value := range fields {
		col := stash.Columns.Find(fieldName)
		if col == nil {
			continue // Column doesn't exist - handled elsewhere
		}
		colResult := ValidateValue(col, value)
		if !colResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, colResult.Errors...)
		}
	}

	// Check required fields that are not being set
	for _, col := range stash.Columns {
		if col.Required {
			if _, ok := fields[col.Name]; !ok {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Column:  col.Name,
					Value:   "",
					Rule:    "required",
					Message: fmt.Sprintf("column '%s' is required", col.Name),
				})
			}
		}
	}

	return result
}

// ValidateStashOutput represents the output of the validate command
type ValidateStashOutput struct {
	Stash       string            `json:"stash"`
	TotalRecords int              `json:"total_records"`
	ValidRecords int              `json:"valid_records"`
	ErrorCount   int              `json:"error_count"`
	Errors       []ValidationError `json:"errors,omitempty"`
}

var validateCmd = &cobra.Command{
	Use:   "validate [stash-name]",
	Short: "Validate all records against column constraints",
	Long: `Validate all records in a stash against column constraints.

Checks all records for:
  - Required field violations
  - Enum value violations
  - Format violations (email, url, number, date)

Examples:
  stash validate
  stash validate inventory
  stash validate --json

AI Agent Examples:
  # Validate before bulk import
  stash validate inventory --json | jq 'select(.error_count > 0)'

  # Check for validation errors
  stash validate --json | jq '.errors[] | select(.rule == "required")'

  # Get count of invalid records
  stash validate --json | jq '.error_count'

Exit Codes:
  0  Success - all records valid
  1  Stash not found
  2  Validation errors found (records fail constraints)

JSON Output (--json):
  {
    "stash": "inventory",
    "total_records": 100,
    "valid_records": 95,
    "error_count": 5,
    "errors": [
      {"column": "email", "value": "invalid", "rule": "email", "message": "...", "record_id": "inv-abc1"}
    ]
  }
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Resolve context - stash is required
	var stashNameArg string
	if len(args) > 0 {
		stashNameArg = args[0]
	}

	ctx, err := context.ResolveRequired(GetActorName(), stashNameArg)
	if err != nil {
		if errors.Is(err, context.ErrNoStashDir) {
			ExitNoStashDir()
			return nil
		}
		if errors.Is(err, context.ErrNoStash) {
			ExitValidationError("no stash specified and multiple stashes exist (use --stash or provide stash name)", nil)
			return nil
		}
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// If stash name was provided as argument, use it
	if stashNameArg != "" {
		ctx.Stash = stashNameArg
	}

	// Create storage
	store, err := storage.NewStore(ctx.StashDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get stash configuration
	stash, err := store.GetStash(ctx.Stash)
	if err != nil {
		if errors.Is(err, model.ErrStashNotFound) {
			ExitStashNotFound(ctx.Stash)
			return nil
		}
		return fmt.Errorf("failed to get stash: %w", err)
	}

	// Get all records
	records, err := store.ListRecords(ctx.Stash, storage.ListOptions{
		ParentID:       "*",
		IncludeDeleted: false,
	})
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// Validate all records
	output := ValidateStashOutput{
		Stash:        ctx.Stash,
		TotalRecords: len(records),
		ValidRecords: 0,
		ErrorCount:   0,
		Errors:       []ValidationError{},
	}

	for _, record := range records {
		result := ValidateRecord(stash, record)
		if result.Valid {
			output.ValidRecords++
		} else {
			output.ErrorCount += len(result.Errors)
			output.Errors = append(output.Errors, result.Errors...)
		}
	}

	// Output result
	if GetJSONOutput() {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		if output.ErrorCount == 0 {
			if !IsQuiet() {
				fmt.Printf("All %d records in stash '%s' are valid\n", output.TotalRecords, ctx.Stash)
			}
		} else {
			fmt.Printf("Validation errors in stash '%s':\n", ctx.Stash)
			fmt.Printf("  Total records: %d\n", output.TotalRecords)
			fmt.Printf("  Valid records: %d\n", output.ValidRecords)
			fmt.Printf("  Errors: %d\n\n", output.ErrorCount)

			for _, err := range output.Errors {
				fmt.Printf("  [%s] %s: %s\n", err.RecordID, err.Column, err.Message)
			}
		}
	}

	// Exit with code 2 if validation errors found
	if output.ErrorCount > 0 {
		Exit(2)
	}

	return nil
}
