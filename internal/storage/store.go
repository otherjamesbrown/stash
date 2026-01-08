package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/user/stash/internal/model"
)

// Store implements the Storage interface using JSONL files and SQLite cache.
type Store struct {
	baseDir string // .stash directory
	jsonl   *JSONLStore
	sqlite  *SQLiteCache
	config  *ConfigStore
}

// NewStore creates a new storage instance.
func NewStore(baseDir string) (*Store, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create stash directory: %w", err)
	}

	jsonl := NewJSONLStore(baseDir)
	config := NewConfigStore(baseDir)

	sqlite, err := NewSQLiteCache(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite cache: %w", err)
	}

	return &Store{
		baseDir: baseDir,
		jsonl:   jsonl,
		sqlite:  sqlite,
		config:  config,
	}, nil
}

// Close releases resources.
func (s *Store) Close() error {
	return s.sqlite.Close()
}

// BaseDir returns the base directory path.
func (s *Store) BaseDir() string {
	return s.baseDir
}

// CreateStash creates a new stash with the given name and prefix.
func (s *Store) CreateStash(name, prefix string, stash *model.Stash) error {
	// Check if stash already exists
	if s.config.Exists(name) {
		return model.ErrStashExists
	}

	// Write config file
	if err := s.config.WriteConfig(stash); err != nil {
		return err
	}

	// Create SQLite table
	if err := s.sqlite.CreateStashTable(stash); err != nil {
		// Rollback config on failure
		s.config.DeleteConfig(name)
		return err
	}

	return nil
}

// DropStash removes a stash and all its data.
func (s *Store) DropStash(name string) error {
	// Check if stash exists
	if !s.config.Exists(name) {
		return model.ErrStashNotFound
	}

	// Drop SQLite table
	if err := s.sqlite.DropStashTable(name); err != nil {
		return err
	}

	// Delete config directory (includes JSONL)
	if err := s.config.DeleteConfig(name); err != nil {
		return err
	}

	return nil
}

// GetStash retrieves stash configuration.
func (s *Store) GetStash(name string) (*model.Stash, error) {
	// Try SQLite cache first
	stash, err := s.sqlite.GetStash(name)
	if err == nil {
		return stash, nil
	}

	// Fall back to config file
	stash, err = s.config.ReadConfig(name)
	if err != nil {
		return nil, err
	}

	return stash, nil
}

// ListStashes returns all stash configurations.
func (s *Store) ListStashes() ([]*model.Stash, error) {
	// Try SQLite cache first
	stashes, err := s.sqlite.ListStashes()
	if err == nil && len(stashes) > 0 {
		return stashes, nil
	}

	// Fall back to config files
	names, err := s.config.ListStashDirs()
	if err != nil {
		return nil, err
	}

	stashes = make([]*model.Stash, 0, len(names))
	for _, name := range names {
		stash, err := s.config.ReadConfig(name)
		if err != nil {
			continue // Skip invalid configs
		}
		stashes = append(stashes, stash)
	}

	return stashes, nil
}

// AddColumn adds a new column to a stash.
func (s *Store) AddColumn(stashName string, col model.Column) error {
	// Get current stash config
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Add column to stash
	if err := stash.AddColumn(col); err != nil {
		return err
	}

	// Update config file
	if err := s.config.WriteConfig(stash); err != nil {
		return err
	}

	// Add column to SQLite table
	if err := s.sqlite.AddColumn(stashName, col.Name); err != nil {
		return err
	}

	// Update SQLite metadata
	if err := s.sqlite.UpdateStashConfig(stash); err != nil {
		return err
	}

	return nil
}

// CreateRecord creates a new record.
func (s *Store) CreateRecord(stashName string, record *model.Record) error {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Set operation type
	record.Operation = model.OpCreate

	// Calculate hash
	record.Hash = record.CalculateHash()

	// Append to JSONL
	if err := s.jsonl.AppendRecord(stashName, record); err != nil {
		return err
	}

	// Update SQLite cache
	columns := stash.Columns.Names()
	if err := s.sqlite.UpsertRecord(stashName, record, columns); err != nil {
		return err
	}

	return nil
}

// UpdateRecord updates an existing record.
func (s *Store) UpdateRecord(stashName string, record *model.Record) error {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Set operation type
	record.Operation = model.OpUpdate

	// Calculate new hash
	record.Hash = record.CalculateHash()

	// Append to JSONL
	if err := s.jsonl.AppendRecord(stashName, record); err != nil {
		return err
	}

	// Update SQLite cache
	columns := stash.Columns.Names()
	if err := s.sqlite.UpsertRecord(stashName, record, columns); err != nil {
		return err
	}

	return nil
}

// DeleteRecord soft-deletes a record.
func (s *Store) DeleteRecord(stashName string, id string, actor string) error {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Get current record
	record, err := s.GetRecord(stashName, id)
	if err != nil {
		return err
	}

	// Check if already deleted
	if record.IsDeleted() {
		return model.ErrRecordDeleted
	}

	// Set deletion metadata
	now := time.Now()
	record.DeletedAt = &now
	record.DeletedBy = actor
	record.UpdatedAt = now
	record.UpdatedBy = actor
	record.Operation = model.OpDelete

	// Append to JSONL
	if err := s.jsonl.AppendRecord(stashName, record); err != nil {
		return err
	}

	// Update SQLite cache
	columns := stash.Columns.Names()
	if err := s.sqlite.UpsertRecord(stashName, record, columns); err != nil {
		return err
	}

	return nil
}

// RestoreRecord restores a soft-deleted record.
func (s *Store) RestoreRecord(stashName string, id string, actor string) error {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Get current record (including deleted)
	record, err := s.GetRecordIncludeDeleted(stashName, id)
	if err != nil {
		return err
	}

	// Check if not deleted
	if !record.IsDeleted() {
		return fmt.Errorf("record is not deleted")
	}

	// Clear deletion metadata
	record.DeletedAt = nil
	record.DeletedBy = ""
	record.UpdatedAt = time.Now()
	record.UpdatedBy = actor
	record.Operation = model.OpRestore

	// Append to JSONL
	if err := s.jsonl.AppendRecord(stashName, record); err != nil {
		return err
	}

	// Update SQLite cache
	columns := stash.Columns.Names()
	if err := s.sqlite.UpsertRecord(stashName, record, columns); err != nil {
		return err
	}

	return nil
}

// GetRecord retrieves a record by ID.
func (s *Store) GetRecord(stashName string, id string) (*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	record, err := s.sqlite.GetRecord(stashName, id, columns)
	if err != nil {
		return nil, err
	}

	// Check if deleted
	if record.IsDeleted() {
		return nil, model.ErrRecordDeleted
	}

	return record, nil
}

// GetRecordIncludeDeleted retrieves a record including soft-deleted ones.
func (s *Store) GetRecordIncludeDeleted(stashName string, id string) (*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	return s.sqlite.GetRecord(stashName, id, columns)
}

// ListRecords lists records with filtering options.
func (s *Store) ListRecords(stashName string, opts ListOptions) ([]*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	return s.sqlite.ListRecords(stashName, columns, opts)
}

// GetChildren returns direct children of a parent record (excluding deleted).
func (s *Store) GetChildren(stashName string, parentID string) ([]*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	return s.sqlite.GetChildren(stashName, parentID, columns)
}

// GetChildrenIncludeDeleted returns direct children of a parent record (including deleted).
func (s *Store) GetChildrenIncludeDeleted(stashName string, parentID string) ([]*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	return s.sqlite.ListRecords(stashName, columns, ListOptions{
		ParentID:       parentID,
		IncludeDeleted: true,
	})
}

// GetNextChildSeq returns the next sequence number for a child record.
func (s *Store) GetNextChildSeq(stashName string, parentID string) (int, error) {
	return s.sqlite.GetNextChildSeq(stashName, parentID)
}

// RebuildCache rebuilds the SQLite cache from JSONL files.
func (s *Store) RebuildCache(stashName string) error {
	stash, err := s.config.ReadConfig(stashName)
	if err != nil {
		return err
	}

	// Clear existing cache
	if err := s.sqlite.ClearTable(stashName); err != nil {
		// Table might not exist, try to create it
		if err := s.sqlite.CreateStashTable(stash); err != nil {
			return err
		}
	}

	// Read all records from JSONL
	records, err := s.jsonl.ReadAllRecords(stashName)
	if err != nil {
		return err
	}

	// Build current state by replaying operations
	state := make(map[string]*model.Record)
	for _, record := range records {
		switch record.Operation {
		case model.OpCreate, model.OpUpdate, model.OpRestore:
			state[record.ID] = record
		case model.OpDelete:
			if existing, ok := state[record.ID]; ok {
				existing.DeletedAt = record.DeletedAt
				existing.DeletedBy = record.DeletedBy
				existing.UpdatedAt = record.UpdatedAt
				existing.UpdatedBy = record.UpdatedBy
			}
		}
	}

	// Insert current state into SQLite
	columns := stash.Columns.Names()
	for _, record := range state {
		if err := s.sqlite.UpsertRecord(stashName, record, columns); err != nil {
			return err
		}
	}

	return nil
}

// FlushToJSONL writes the current SQLite state to a new JSONL file.
// This compacts the log by removing historical operations.
func (s *Store) FlushToJSONL(stashName string) error {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return err
	}

	// Get all records from SQLite (including deleted)
	columns := stash.Columns.Names()
	records, err := s.sqlite.ListRecords(stashName, columns, ListOptions{
		IncludeDeleted: true,
		ParentID:       "*", // All records
	})
	if err != nil {
		return err
	}

	// Set operation to create for compacted log
	for _, record := range records {
		if record.IsDeleted() {
			record.Operation = model.OpDelete
		} else {
			record.Operation = model.OpCreate
		}
	}

	// Write all records atomically
	if err := s.jsonl.WriteAllRecords(stashName, records); err != nil {
		return err
	}

	return nil
}

// CountRecords returns the number of records in a stash (excluding deleted).
func (s *Store) CountRecords(stashName string) (int, error) {
	return s.sqlite.CountRecords(stashName)
}

// PurgeRecord permanently removes a soft-deleted record from both SQLite and JSONL.
func (s *Store) PurgeRecord(stashName string, id string) error {
	// Get record (must be deleted)
	record, err := s.GetRecordIncludeDeleted(stashName, id)
	if err != nil {
		return err
	}

	if !record.IsDeleted() {
		return fmt.Errorf("record '%s' is not deleted; cannot purge active records", id)
	}

	// Delete from SQLite cache
	if err := s.sqlite.DeleteRecord(stashName, id); err != nil {
		return err
	}

	// Note: We don't remove from JSONL here for append-only safety.
	// The FlushToJSONL function will clean up purged records on compaction.

	// Delete associated files
	filesDir := s.GetFilesDir(stashName, id)
	if _, err := os.Stat(filesDir); err == nil {
		if err := os.RemoveAll(filesDir); err != nil {
			// Non-fatal: log warning but continue
		}
	}

	return nil
}

// ListDeletedRecords returns all soft-deleted records, optionally filtered by deletion time.
func (s *Store) ListDeletedRecords(stashName string, before *time.Time) ([]*model.Record, error) {
	stash, err := s.GetStash(stashName)
	if err != nil {
		return nil, err
	}

	columns := stash.Columns.Names()
	records, err := s.sqlite.ListRecords(stashName, columns, ListOptions{
		ParentID:       "*",
		IncludeDeleted: true,
	})
	if err != nil {
		return nil, err
	}

	// Filter to only deleted records
	var deleted []*model.Record
	for _, rec := range records {
		if !rec.IsDeleted() {
			continue
		}
		// Filter by deletion time if specified
		if before != nil && rec.DeletedAt.After(*before) {
			continue
		}
		deleted = append(deleted, rec)
	}

	return deleted, nil
}

// UpdateStashConfig updates the stash configuration in both config file and SQLite.
func (s *Store) UpdateStashConfig(stash *model.Stash) error {
	// Update config file
	if err := s.config.WriteConfig(stash); err != nil {
		return err
	}

	// Update SQLite metadata
	if err := s.sqlite.UpdateStashConfig(stash); err != nil {
		return err
	}

	return nil
}

// GetLastSyncTime returns the last sync time from metadata.
func (s *Store) GetLastSyncTime() (time.Time, error) {
	return s.sqlite.GetLastSyncTime()
}

// DefaultBaseDir returns the default .stash directory path.
func DefaultBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".stash"
	}
	return filepath.Join(home, ".stash")
}

// GetFilesDir returns the path to the files directory for a record.
func (s *Store) GetFilesDir(stashName, recordID string) string {
	return filepath.Join(s.baseDir, stashName, "files", recordID)
}

// AttachFile attaches a file to a record.
// If move is true, the source file is moved; otherwise it's copied.
func (s *Store) AttachFile(stashName, recordID, srcPath string, move bool, actor string) (*model.Attachment, error) {
	// Verify record exists
	_, err := s.GetRecord(stashName, recordID)
	if err != nil {
		return nil, err
	}

	// Get file info
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	// Calculate file hash
	hash, err := model.CalculateFileHash(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Create files directory for record
	filesDir := s.GetFilesDir(stashName, recordID)
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create files directory: %w", err)
	}

	// Check if attachment with same name exists
	destPath := filepath.Join(filesDir, srcInfo.Name())
	if _, err := os.Stat(destPath); err == nil {
		return nil, model.ErrAttachmentExists
	}

	// Copy or move the file
	if move {
		if err := os.Rename(srcPath, destPath); err != nil {
			// Fall back to copy+delete for cross-device moves
			if err := copyFile(srcPath, destPath); err != nil {
				return nil, fmt.Errorf("failed to copy file: %w", err)
			}
			if err := os.Remove(srcPath); err != nil {
				// Non-fatal: file copied but original couldn't be removed
			}
		}
	} else {
		if err := copyFile(srcPath, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	// Create attachment metadata
	attachment := &model.Attachment{
		Name:       srcInfo.Name(),
		Size:       srcInfo.Size(),
		Hash:       hash,
		AttachedAt: time.Now(),
		AttachedBy: actor,
	}

	return attachment, nil
}

// DetachFile removes an attachment from a record.
func (s *Store) DetachFile(stashName, recordID, filename string) error {
	// Verify record exists
	_, err := s.GetRecord(stashName, recordID)
	if err != nil {
		return err
	}

	// Check if file exists
	filePath := filepath.Join(s.GetFilesDir(stashName, recordID), filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return model.ErrAttachmentNotFound
	}

	// Remove the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove attachment: %w", err)
	}

	// Remove directory if empty
	filesDir := s.GetFilesDir(stashName, recordID)
	entries, _ := os.ReadDir(filesDir)
	if len(entries) == 0 {
		os.Remove(filesDir)
	}

	return nil
}

// ListAttachments returns all attachments for a record.
func (s *Store) ListAttachments(stashName, recordID string) ([]*model.Attachment, error) {
	// Verify record exists
	_, err := s.GetRecord(stashName, recordID)
	if err != nil {
		return nil, err
	}

	filesDir := s.GetFilesDir(stashName, recordID)
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Attachment{}, nil
		}
		return nil, fmt.Errorf("failed to read files directory: %w", err)
	}

	attachments := make([]*model.Attachment, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(filesDir, entry.Name())
		hash, _ := model.CalculateFileHash(filePath)

		attachments = append(attachments, &model.Attachment{
			Name:       entry.Name(),
			Size:       info.Size(),
			Hash:       hash,
			AttachedAt: info.ModTime(), // Use mod time as approximate attach time
			AttachedBy: "",             // Unknown from filesystem alone
		})
	}

	return attachments, nil
}

// GetAttachment returns a specific attachment by filename.
func (s *Store) GetAttachment(stashName, recordID, filename string) (*model.Attachment, error) {
	// Verify record exists
	_, err := s.GetRecord(stashName, recordID)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(s.GetFilesDir(stashName, recordID), filename)
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.ErrAttachmentNotFound
		}
		return nil, fmt.Errorf("failed to stat attachment: %w", err)
	}

	hash, _ := model.CalculateFileHash(filePath)

	return &model.Attachment{
		Name:       info.Name(),
		Size:       info.Size(),
		Hash:       hash,
		AttachedAt: info.ModTime(),
		AttachedBy: "",
	}, nil
}

// RawQuery executes a raw SQL SELECT query against the cache.
// Returns rows as a slice of maps and the column names in order.
func (s *Store) RawQuery(query string) ([]map[string]interface{}, []string, error) {
	return s.sqlite.RawQuery(query)
}

// GetRecordHistory retrieves all historical changes for a record from JSONL.
func (s *Store) GetRecordHistory(stashName string, recordID string) ([]*model.Record, error) {
	// Read all records from JSONL
	records, err := s.jsonl.ReadAllRecords(stashName)
	if err != nil {
		return nil, err
	}

	// Filter to only records with matching ID
	var history []*model.Record
	for _, rec := range records {
		if rec.ID == recordID {
			history = append(history, rec)
		}
	}

	return history, nil
}

// GetAllHistory retrieves all historical changes from JSONL.
func (s *Store) GetAllHistory(stashName string) ([]*model.Record, error) {
	return s.jsonl.ReadAllRecords(stashName)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		os.Remove(dst)
		return err
	}

	return dstFile.Sync()
}
