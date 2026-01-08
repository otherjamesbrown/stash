package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/stash/internal/model"
)

// JSONLStore provides append-only JSONL storage for records.
type JSONLStore struct {
	baseDir string // .stash directory
}

// NewJSONLStore creates a new JSONL store.
func NewJSONLStore(baseDir string) *JSONLStore {
	return &JSONLStore{baseDir: baseDir}
}

// getRecordsPath returns the path to records.jsonl for a stash.
func (s *JSONLStore) getRecordsPath(stashName string) string {
	return filepath.Join(s.baseDir, stashName, "records.jsonl")
}

// ensureStashDir ensures the stash directory exists.
func (s *JSONLStore) ensureStashDir(stashName string) error {
	dir := filepath.Join(s.baseDir, stashName)
	return os.MkdirAll(dir, 0755)
}

// AppendRecord appends a record to the JSONL file atomically.
// The file is created if it doesn't exist.
func (s *JSONLStore) AppendRecord(stashName string, record *model.Record) error {
	if err := s.ensureStashDir(stashName); err != nil {
		return fmt.Errorf("failed to create stash directory: %w", err)
	}

	recordsPath := s.getRecordsPath(stashName)

	// Marshal record to JSON
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}
	data = append(data, '\n')

	// Write to temp file first for atomicity
	dir := filepath.Dir(recordsPath)
	tmpFile, err := os.CreateTemp(dir, "records-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on error

	// Copy existing content if file exists
	if existingFile, err := os.Open(recordsPath); err == nil {
		_, copyErr := tmpFile.ReadFrom(existingFile)
		existingFile.Close()
		if copyErr != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to copy existing records: %w", copyErr)
		}
	}

	// Append new record
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write record: %w", err)
	}

	// Sync and close
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, recordsPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ReadAllRecords reads all records from the JSONL file.
// Returns an empty slice if the file doesn't exist.
func (s *JSONLStore) ReadAllRecords(stashName string) ([]*model.Record, error) {
	recordsPath := s.getRecordsPath(stashName)

	file, err := os.Open(recordsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Record{}, nil
		}
		return nil, fmt.Errorf("failed to open records file: %w", err)
	}
	defer file.Close()

	var records []*model.Record
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var record model.Record
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("failed to parse record at line %d: %w", lineNum, err)
		}
		records = append(records, &record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading records file: %w", err)
	}

	return records, nil
}

// WriteAllRecords overwrites the JSONL file with the given records.
// This is used during sync operations.
func (s *JSONLStore) WriteAllRecords(stashName string, records []*model.Record) error {
	if err := s.ensureStashDir(stashName); err != nil {
		return fmt.Errorf("failed to create stash directory: %w", err)
	}

	recordsPath := s.getRecordsPath(stashName)
	dir := filepath.Dir(recordsPath)

	// Write to temp file
	tmpFile, err := os.CreateTemp(dir, "records-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	writer := bufio.NewWriter(tmpFile)
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to marshal record: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write record: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, recordsPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// DeleteFile removes the records.jsonl file for a stash.
func (s *JSONLStore) DeleteFile(stashName string) error {
	recordsPath := s.getRecordsPath(stashName)
	err := os.Remove(recordsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete records file: %w", err)
	}
	return nil
}

// Exists returns true if the records file exists.
func (s *JSONLStore) Exists(stashName string) bool {
	recordsPath := s.getRecordsPath(stashName)
	_, err := os.Stat(recordsPath)
	return err == nil
}
