# Project Stash: Go Implementation

**Version:** 1.0.0
**Language:** Go (Golang)
**Concept:** A "Headless CMS" for AI Agents (Dual Write: File + SQLite)

---

## 1. Technical Specification

### Overview
**Stash** is a lightweight, single-binary tool designed to standardise the "memory" of an AI agent. It allows the agent to write unstructured research (Markdown files) while simultaneously forcing structured metadata into a queryable cache (SQLite).

### Architecture
The tool follows a **Dual Write** pattern:
1.  **File System (Permanent Store):** Raw Markdown files and a simplified JSONL log (`_metadata.jsonl`) serve as the portable, human-readable source of truth.
2.  **SQLite (Ephemeral Cache):** A local database (`cache.db`) is rebuilt or updated on every write to allow for complex SQL querying (e.g., counting, filtering) which LLMs struggle to do with raw files.

### Schema Evolution (The "Magic")
Unlike a traditional app with a fixed schema, Stash is **fluid**.
* If the AI invents a new metadata key (e.g., `"ceo_name": "Satya Nadella"`), Stash detects that the column `ceo_name` is missing from the SQLite table.
* It automatically executes an `ALTER TABLE` command to add the column before inserting the data.
* This prevents the agent from crashing due to schema violations.

---

## 2. Implementation (`stash.go`)

Save the following code as `stash.go`.

```go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (CGO-free)
)

const (
	DataDir   = "data"
	JsonlFile = "_metadata.jsonl"
	DbFile    = "cache.db"
	TableName = "items"
)

// -- HELPER FUNCTIONS --

// ensureDir creates the data directory if it doesn't exist
func ensureDir() {
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		os.Mkdir(DataDir, 0755)
	}
}

// sanitize prevents basic SQL injection on column names
// It strips non-alphanumeric characters (underscores allowed)
func sanitize(input string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, input)
}

// -- CORE LOGIC --

// syncSchema inspects the incoming metadata keys and adds missing columns to SQLite
func syncSchema(db *sql.DB, metadata map[string]interface{}) error {
	// 1. Create table if not exists (Primary Key is always 'filename')
	createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (filename TEXT PRIMARY KEY);`, TableName)
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// 2. Get existing columns via PRAGMA
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", TableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	existingCols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltVal interface{}
		rows.Scan(&cid, &name, &ctype, &notnull, &dfltVal, &pk)
		existingCols[name] = true
	}

	// 3. Compare and Alter Table if key is missing
	for key := range metadata {
		sanitizedKey := sanitize(key)
		if sanitizedKey == "" {
			continue 
		}
		if !existingCols[sanitizedKey] {
			// We default to TEXT for flexibility. SQLite is loosely typed.
			alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT;", TableName, sanitizedKey)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add column %s: %v", sanitizedKey, err)
			}
		}
	}
	return nil
}

// addItem handles the Dual Write logic (File + JSONL + SQLite)
func addItem(filename, content, metadataJSON string) string {
	ensureDir()

	// 1. Parse Metadata
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &meta); err != nil {
		return "Error: Invalid JSON metadata string."
	}
	// Enforce filename as the primary key in metadata
	meta["filename"] = filename

	// 2. Write Markdown File (The Content)
	mdPath := filepath.Join(DataDir, filename)
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error writing MD file: %v", err)
	}

	// 3. Append to JSONL (The Log)
	jsonlPath := filepath.Join(DataDir, JsonlFile)
	f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("Error opening JSONL: %v", err)
	}
	defer f.Close()
	
	fullRecord, _ := json.Marshal(meta)
	if _, err := f.WriteString(string(fullRecord) + "\n"); err != nil {
		return fmt.Sprintf("Error writing JSONL: %v", err)
	}

	// 4. Update SQLite (The Cache)
	db, err := sql.Open("sqlite", DbFile)
	if err != nil {
		return fmt.Sprintf("Error opening DB: %v", err)
	}
	defer db.Close()

	if err := syncSchema(db, meta); err != nil {
		return fmt.Sprintf("DB Schema Error: %v", err)
	}

	// Construct Dynamic Insert/Upsert Query
	cols := []string{}
	vals := []interface{}{}
	placeholders := []string{}

	for k, v := range meta {
		safeKey := sanitize(k)
		if safeKey != "" {
			cols = append(cols, safeKey)
			vals = append(vals, v)
			placeholders = append(placeholders, "?")
		}
	}

	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)", 
		TableName, 
		strings.Join(cols, ","), 
		strings.Join(placeholders, ","))

	if _, err := db.Exec(query, vals...); err != nil {
		return fmt.Sprintf("DB Write Error: %v", err)
	}

	return fmt.Sprintf("Success: Stashed %s and updated DB.", filename)
}

// queryDB executes a read-only SQL query and returns JSON
func queryDB(sqlQuery string) string {
	db, err := sql.Open("sqlite", DbFile)
	if err != nil {
		return fmt.Sprintf("Error opening DB: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(sqlQuery)
	if err != nil {
		return fmt.Sprintf("SQL Error: %v", err)
	}
	defer rows.Close()

	// Dynamic result parsing (scan into interface{})
	columns, _ := rows.Columns()
	count := len(columns)
	tableData := []map[string]interface{}{}

	for rows.Next() {
		values := make([]interface{}, count)
		valuePtrs := make([]interface{}, count)
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			// SQLite driver often returns []byte for text
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		tableData = append(tableData, entry)
	}

	// If empty, return friendly message or empty list
	if len(tableData) == 0 {
		return "[]"
	}

	result, _ := json.MarshalIndent(tableData, "", "  ")
	return string(result)
}

// -- CLI ENTRY POINT --

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  stash add <filename> <content> <json_metadata>")
		fmt.Println("  stash query <sql_query>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	if cmd == "add" && len(os.Args) >= 5 {
		// usage: ./stash add "file.md" "# Title" '{"key":"val"}'
		fmt.Println(addItem(os.Args[2], os.Args[3], os.Args[4]))
	} else if cmd == "query" && len(os.Args) >= 3 {
		// usage: ./stash query "SELECT * FROM items"
		fmt.Println(queryDB(os.Args[2]))
	} else {
		fmt.Println("Invalid arguments or command.")
	}
}
