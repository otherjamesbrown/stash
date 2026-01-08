package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/user/stash/internal/model"
)

// SQLiteCache provides SQLite-based caching for fast queries.
type SQLiteCache struct {
	db      *sql.DB
	dbPath  string
	baseDir string // .stash directory
}

// NewSQLiteCache creates a new SQLite cache.
func NewSQLiteCache(baseDir string) (*SQLiteCache, error) {
	dbPath := filepath.Join(baseDir, "cache.db")

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	cache := &SQLiteCache{
		db:      db,
		dbPath:  dbPath,
		baseDir: baseDir,
	}

	if err := cache.initMetaTable(); err != nil {
		db.Close()
		return nil, err
	}

	return cache, nil
}

// initMetaTable creates the metadata table if it doesn't exist.
func (c *SQLiteCache) initMetaTable() error {
	_, err := c.db.Exec(`
		CREATE TABLE IF NOT EXISTS _stash_meta (
			stash_name TEXT PRIMARY KEY,
			prefix TEXT,
			config_json TEXT,
			last_sync TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create meta table: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (c *SQLiteCache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// sanitizeTableName converts stash name to a safe table name.
func sanitizeTableName(name string) string {
	// Replace hyphens with underscores, SQLite identifiers can't have hyphens
	safe := strings.ReplaceAll(name, "-", "_")
	return safe
}

// CreateStashTable creates a table for a stash with the base schema.
func (c *SQLiteCache) CreateStashTable(stash *model.Stash) error {
	tableName := sanitizeTableName(stash.Name)

	// Create main table
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s" (
			id TEXT PRIMARY KEY,
			hash TEXT NOT NULL,
			parent_id TEXT,
			created_at TEXT NOT NULL,
			created_by TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			updated_by TEXT NOT NULL,
			branch TEXT,
			deleted_at TEXT,
			deleted_by TEXT
		)
	`, tableName)

	if _, err := c.db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create stash table: %w", err)
	}

	// Create indexes
	indexes := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_parent" ON "%s"(parent_id)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_deleted" ON "%s"(deleted_at)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_hash" ON "%s"(hash)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_branch" ON "%s"(branch)`, tableName, tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_updated" ON "%s"(updated_at)`, tableName, tableName),
	}

	for _, idx := range indexes {
		if _, err := c.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Add columns for existing schema
	for _, col := range stash.Columns {
		if err := c.AddColumn(stash.Name, col.Name); err != nil {
			return err
		}
	}

	// Store metadata
	configJSON, err := json.Marshal(stash)
	if err != nil {
		return fmt.Errorf("failed to marshal stash config: %w", err)
	}

	_, err = c.db.Exec(`
		INSERT OR REPLACE INTO _stash_meta (stash_name, prefix, config_json, last_sync)
		VALUES (?, ?, ?, ?)
	`, stash.Name, stash.Prefix, string(configJSON), time.Now().Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("failed to store stash metadata: %w", err)
	}

	return nil
}

// DropStashTable drops the table for a stash.
func (c *SQLiteCache) DropStashTable(stashName string) error {
	tableName := sanitizeTableName(stashName)

	if _, err := c.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)); err != nil {
		return fmt.Errorf("failed to drop stash table: %w", err)
	}

	if _, err := c.db.Exec(`DELETE FROM _stash_meta WHERE stash_name = ?`, stashName); err != nil {
		return fmt.Errorf("failed to delete stash metadata: %w", err)
	}

	return nil
}

// AddColumn adds a new column to a stash table.
func (c *SQLiteCache) AddColumn(stashName, columnName string) error {
	tableName := sanitizeTableName(stashName)

	// SQLite ALTER TABLE doesn't support IF NOT EXISTS for columns,
	// so we check if column exists first
	exists, err := c.columnExists(tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil // Column already exists
	}

	alterSQL := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" TEXT`, tableName, columnName)
	if _, err := c.db.Exec(alterSQL); err != nil {
		return fmt.Errorf("failed to add column %s: %w", columnName, err)
	}

	return nil
}

// columnExists checks if a column exists in a table.
func (c *SQLiteCache) columnExists(tableName, columnName string) (bool, error) {
	rows, err := c.db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		return false, fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}

	return false, rows.Err()
}

// GetStash retrieves stash configuration from metadata.
func (c *SQLiteCache) GetStash(name string) (*model.Stash, error) {
	var configJSON string
	err := c.db.QueryRow(`SELECT config_json FROM _stash_meta WHERE stash_name = ?`, name).Scan(&configJSON)
	if err == sql.ErrNoRows {
		return nil, model.ErrStashNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stash: %w", err)
	}

	var stash model.Stash
	if err := json.Unmarshal([]byte(configJSON), &stash); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stash config: %w", err)
	}

	return &stash, nil
}

// UpdateStashConfig updates the stash configuration in metadata.
func (c *SQLiteCache) UpdateStashConfig(stash *model.Stash) error {
	configJSON, err := json.Marshal(stash)
	if err != nil {
		return fmt.Errorf("failed to marshal stash config: %w", err)
	}

	result, err := c.db.Exec(`
		UPDATE _stash_meta SET config_json = ?, last_sync = ? WHERE stash_name = ?
	`, string(configJSON), time.Now().Format(time.RFC3339), stash.Name)
	if err != nil {
		return fmt.Errorf("failed to update stash config: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return model.ErrStashNotFound
	}

	return nil
}

// ListStashes returns all stash configurations.
func (c *SQLiteCache) ListStashes() ([]*model.Stash, error) {
	rows, err := c.db.Query(`SELECT config_json FROM _stash_meta ORDER BY stash_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list stashes: %w", err)
	}
	defer rows.Close()

	var stashes []*model.Stash
	for rows.Next() {
		var configJSON string
		if err := rows.Scan(&configJSON); err != nil {
			return nil, err
		}

		var stash model.Stash
		if err := json.Unmarshal([]byte(configJSON), &stash); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stash config: %w", err)
		}
		stashes = append(stashes, &stash)
	}

	return stashes, rows.Err()
}

// UpsertRecord inserts or updates a record in the cache.
func (c *SQLiteCache) UpsertRecord(stashName string, record *model.Record, columns []string) error {
	tableName := sanitizeTableName(stashName)

	// Build column list
	baseCols := []string{"id", "hash", "parent_id", "created_at", "created_by", "updated_at", "updated_by", "branch", "deleted_at", "deleted_by"}
	allCols := append(baseCols, columns...)

	// Build placeholders
	placeholders := make([]string, len(allCols))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	// Build values
	var deletedAt, deletedBy interface{}
	if record.DeletedAt != nil {
		deletedAt = record.DeletedAt.Format(time.RFC3339)
		deletedBy = record.DeletedBy
	}

	values := []interface{}{
		record.ID,
		record.Hash,
		nullString(record.ParentID),
		record.CreatedAt.Format(time.RFC3339),
		record.CreatedBy,
		record.UpdatedAt.Format(time.RFC3339),
		record.UpdatedBy,
		nullString(record.Branch),
		deletedAt,
		deletedBy,
	}

	// Add user field values
	for _, col := range columns {
		if v, ok := record.Fields[col]; ok {
			// Convert to string for storage
			switch val := v.(type) {
			case string:
				values = append(values, val)
			case nil:
				values = append(values, nil)
			default:
				// JSON encode non-string values
				jsonVal, _ := json.Marshal(val)
				values = append(values, string(jsonVal))
			}
		} else {
			values = append(values, nil)
		}
	}

	// Quote column names
	quotedCols := make([]string, len(allCols))
	for i, col := range allCols {
		quotedCols[i] = fmt.Sprintf(`"%s"`, col)
	}

	sql := fmt.Sprintf(`
		INSERT OR REPLACE INTO "%s" (%s) VALUES (%s)
	`, tableName, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	_, err := c.db.Exec(sql, values...)
	if err != nil {
		return fmt.Errorf("failed to upsert record: %w", err)
	}

	return nil
}

// GetRecord retrieves a record from the cache.
func (c *SQLiteCache) GetRecord(stashName, id string, columns []string) (*model.Record, error) {
	tableName := sanitizeTableName(stashName)

	// Build column list
	baseCols := []string{"id", "hash", "parent_id", "created_at", "created_by", "updated_at", "updated_by", "branch", "deleted_at", "deleted_by"}
	allCols := append(baseCols, columns...)

	quotedCols := make([]string, len(allCols))
	for i, col := range allCols {
		quotedCols[i] = fmt.Sprintf(`"%s"`, col)
	}

	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE id = ?`, strings.Join(quotedCols, ", "), tableName)

	row := c.db.QueryRow(query, id)

	record, err := c.scanRecord(row, columns)
	if err == sql.ErrNoRows {
		return nil, model.ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	return record, nil
}

// DeleteRecord removes a record from the cache (hard delete).
func (c *SQLiteCache) DeleteRecord(stashName, id string) error {
	tableName := sanitizeTableName(stashName)

	_, err := c.db.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, tableName), id)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}

// ListRecords lists records from the cache with filtering options.
func (c *SQLiteCache) ListRecords(stashName string, columns []string, opts ListOptions) ([]*model.Record, error) {
	tableName := sanitizeTableName(stashName)

	// Build column list
	baseCols := []string{"id", "hash", "parent_id", "created_at", "created_by", "updated_at", "updated_by", "branch", "deleted_at", "deleted_by"}
	allCols := append(baseCols, columns...)

	quotedCols := make([]string, len(allCols))
	for i, col := range allCols {
		quotedCols[i] = fmt.Sprintf(`"%s"`, col)
	}

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	// Handle deleted record filtering
	if opts.DeletedOnly {
		conditions = append(conditions, "deleted_at IS NOT NULL")
	} else if !opts.IncludeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}

	if opts.ParentID != "*" {
		if opts.ParentID == "" {
			conditions = append(conditions, "parent_id IS NULL")
		} else {
			conditions = append(conditions, "parent_id = ?")
			args = append(args, opts.ParentID)
		}
	}

	// Add WHERE conditions
	for _, w := range opts.Where {
		// Resolve field name case-insensitively
		fieldName := c.resolveColumnName(tableName, w.Field, columns)
		if fieldName == "" {
			fieldName = w.Field // Use as-is if not found
		}

		switch w.Operator {
		case "=":
			conditions = append(conditions, fmt.Sprintf(`"%s" = ?`, fieldName))
			args = append(args, w.Value)
		case "!=", "<>":
			conditions = append(conditions, fmt.Sprintf(`"%s" != ?`, fieldName))
			args = append(args, w.Value)
		case "<":
			conditions = append(conditions, fmt.Sprintf(`CAST("%s" AS REAL) < CAST(? AS REAL)`, fieldName))
			args = append(args, w.Value)
		case ">":
			conditions = append(conditions, fmt.Sprintf(`CAST("%s" AS REAL) > CAST(? AS REAL)`, fieldName))
			args = append(args, w.Value)
		case "<=":
			conditions = append(conditions, fmt.Sprintf(`CAST("%s" AS REAL) <= CAST(? AS REAL)`, fieldName))
			args = append(args, w.Value)
		case ">=":
			conditions = append(conditions, fmt.Sprintf(`CAST("%s" AS REAL) >= CAST(? AS REAL)`, fieldName))
			args = append(args, w.Value)
		case "LIKE":
			conditions = append(conditions, fmt.Sprintf(`"%s" LIKE ?`, fieldName))
			args = append(args, w.Value)
		}
	}

	// Add search condition (search across all user columns)
	if opts.Search != "" {
		var searchConds []string
		searchPattern := "%" + opts.Search + "%"
		for _, col := range columns {
			searchConds = append(searchConds, fmt.Sprintf(`"%s" LIKE ?`, col))
			args = append(args, searchPattern)
		}
		// Also search ID
		searchConds = append(searchConds, `"id" LIKE ?`)
		args = append(args, searchPattern)

		if len(searchConds) > 0 {
			conditions = append(conditions, "("+strings.Join(searchConds, " OR ")+")")
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Build ORDER BY clause
	orderBy := "updated_at"
	if opts.OrderBy != "" {
		// Resolve order by field name case-insensitively
		resolvedOrderBy := c.resolveColumnName(tableName, opts.OrderBy, columns)
		if resolvedOrderBy != "" {
			orderBy = resolvedOrderBy
		} else {
			orderBy = opts.OrderBy
		}
	}
	orderDir := "ASC"
	if opts.Descending {
		orderDir = "DESC"
	}

	query := fmt.Sprintf(`SELECT %s FROM "%s" %s ORDER BY "%s" %s`,
		strings.Join(quotedCols, ", "), tableName, whereClause, orderBy, orderDir)

	// Add LIMIT and OFFSET
	// SQLite requires LIMIT before OFFSET, and OFFSET requires LIMIT
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	} else if opts.Offset > 0 {
		// If only offset is specified, use -1 for unlimited
		query += fmt.Sprintf(" LIMIT -1 OFFSET %d", opts.Offset)
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer rows.Close()

	var records []*model.Record
	for rows.Next() {
		record, err := c.scanRecordFromRows(rows, columns)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// resolveColumnName finds the actual column name case-insensitively.
func (c *SQLiteCache) resolveColumnName(tableName, fieldName string, columns []string) string {
	fieldLower := strings.ToLower(fieldName)

	// Check system columns
	systemCols := []string{"id", "hash", "parent_id", "created_at", "created_by", "updated_at", "updated_by", "branch", "deleted_at", "deleted_by"}
	for _, col := range systemCols {
		if strings.ToLower(col) == fieldLower {
			return col
		}
	}

	// Check user columns
	for _, col := range columns {
		if strings.ToLower(col) == fieldLower {
			return col
		}
	}

	return ""
}

// GetChildren returns direct children of a parent record.
func (c *SQLiteCache) GetChildren(stashName, parentID string, columns []string) ([]*model.Record, error) {
	return c.ListRecords(stashName, columns, ListOptions{
		ParentID:       parentID,
		IncludeDeleted: false,
	})
}

// GetNextChildSeq returns the next sequence number for a child record.
func (c *SQLiteCache) GetNextChildSeq(stashName, parentID string) (int, error) {
	tableName := sanitizeTableName(stashName)

	var maxSeq sql.NullInt64
	query := fmt.Sprintf(`
		SELECT MAX(CAST(SUBSTR(id, LENGTH(?) + 2) AS INTEGER))
		FROM "%s"
		WHERE id LIKE ? || '.%%' AND id NOT LIKE ? || '.%%.%%'
	`, tableName)

	err := c.db.QueryRow(query, parentID, parentID, parentID).Scan(&maxSeq)
	if err != nil {
		return 1, fmt.Errorf("failed to get max child seq: %w", err)
	}

	if !maxSeq.Valid {
		return 1, nil
	}

	return int(maxSeq.Int64) + 1, nil
}

// ClearTable removes all records from a stash table.
func (c *SQLiteCache) ClearTable(stashName string) error {
	tableName := sanitizeTableName(stashName)
	_, err := c.db.Exec(fmt.Sprintf(`DELETE FROM "%s"`, tableName))
	if err != nil {
		return fmt.Errorf("failed to clear table: %w", err)
	}
	return nil
}

// scanRecord scans a single row into a Record.
func (c *SQLiteCache) scanRecord(row *sql.Row, columns []string) (*model.Record, error) {
	var (
		id, hash, createdBy, updatedBy string
		parentID, branch               sql.NullString
		createdAt, updatedAt           string
		deletedAt, deletedBy           sql.NullString
	)

	// Prepare slice for user columns
	userVals := make([]sql.NullString, len(columns))
	userPtrs := make([]interface{}, len(columns))
	for i := range userVals {
		userPtrs[i] = &userVals[i]
	}

	// Build scan destinations
	dests := []interface{}{
		&id, &hash, &parentID, &createdAt, &createdBy,
		&updatedAt, &updatedBy, &branch, &deletedAt, &deletedBy,
	}
	dests = append(dests, userPtrs...)

	if err := row.Scan(dests...); err != nil {
		return nil, err
	}

	return c.buildRecord(id, hash, parentID, createdAt, createdBy, updatedAt, updatedBy, branch, deletedAt, deletedBy, columns, userVals)
}

// scanRecordFromRows scans a row from Rows into a Record.
func (c *SQLiteCache) scanRecordFromRows(rows *sql.Rows, columns []string) (*model.Record, error) {
	var (
		id, hash, createdBy, updatedBy string
		parentID, branch               sql.NullString
		createdAt, updatedAt           string
		deletedAt, deletedBy           sql.NullString
	)

	// Prepare slice for user columns
	userVals := make([]sql.NullString, len(columns))
	userPtrs := make([]interface{}, len(columns))
	for i := range userVals {
		userPtrs[i] = &userVals[i]
	}

	// Build scan destinations
	dests := []interface{}{
		&id, &hash, &parentID, &createdAt, &createdBy,
		&updatedAt, &updatedBy, &branch, &deletedAt, &deletedBy,
	}
	dests = append(dests, userPtrs...)

	if err := rows.Scan(dests...); err != nil {
		return nil, err
	}

	return c.buildRecord(id, hash, parentID, createdAt, createdBy, updatedAt, updatedBy, branch, deletedAt, deletedBy, columns, userVals)
}

// buildRecord constructs a Record from scanned values.
func (c *SQLiteCache) buildRecord(
	id, hash string,
	parentID sql.NullString,
	createdAt, createdBy string,
	updatedAt, updatedBy string,
	branch sql.NullString,
	deletedAt, deletedBy sql.NullString,
	columns []string,
	userVals []sql.NullString,
) (*model.Record, error) {
	record := &model.Record{
		ID:        id,
		Hash:      hash,
		CreatedBy: createdBy,
		UpdatedBy: updatedBy,
		Fields:    make(map[string]interface{}),
	}

	if parentID.Valid {
		record.ParentID = parentID.String
	}
	if branch.Valid {
		record.Branch = branch.String
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		record.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		record.UpdatedAt = t
	}
	if deletedAt.Valid {
		if t, err := time.Parse(time.RFC3339, deletedAt.String); err == nil {
			record.DeletedAt = &t
		}
		record.DeletedBy = deletedBy.String
	}

	// Set user fields
	for i, col := range columns {
		if userVals[i].Valid {
			// Try to unmarshal JSON, fall back to string
			var val interface{}
			if err := json.Unmarshal([]byte(userVals[i].String), &val); err != nil {
				val = userVals[i].String
			}
			record.Fields[col] = val
		}
	}

	return record, nil
}

// TableExists checks if a stash table exists.
func (c *SQLiteCache) TableExists(stashName string) (bool, error) {
	tableName := sanitizeTableName(stashName)
	var name string
	err := c.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CountRecords returns the number of non-deleted records in a stash.
func (c *SQLiteCache) CountRecords(stashName string) (int, error) {
	tableName := sanitizeTableName(stashName)

	var count int
	err := c.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE deleted_at IS NULL`, tableName)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count records: %w", err)
	}
	return count, nil
}

// GetLastSyncTime returns the most recent last_sync time from all stashes.
func (c *SQLiteCache) GetLastSyncTime() (time.Time, error) {
	var lastSyncStr sql.NullString
	err := c.db.QueryRow(`SELECT MAX(last_sync) FROM _stash_meta`).Scan(&lastSyncStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last sync time: %w", err)
	}

	if !lastSyncStr.Valid || lastSyncStr.String == "" {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339, lastSyncStr.String)
}

// nullString converts empty string to sql.NullString.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// RawQuery executes a raw SQL SELECT query and returns results.
// Only SELECT queries should be passed to this function.
func (c *SQLiteCache) RawQuery(query string) ([]map[string]interface{}, []string, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Prepare result slice
	var results []map[string]interface{}

	// Prepare scan destinations
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for readability
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return results, columns, nil
}
