package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// executor defines an interface for executing SQL queries, compatible with *sql.DB and *sql.Tx.
type executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// DB wraps sql.DB with our application-specific methods
type DB struct {
	*sql.DB
	verbose bool
}

// New creates a new database connection and initializes the schema
func New(dbPath string, verbose bool) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening database at %s: %w", dbPath, err)
	}

	dbWrapper := &DB{
		DB:      db,
		verbose: verbose,
	}

	if err := dbWrapper.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing database schema: %w", err)
	}

	return dbWrapper, nil
}

// initSchema creates the necessary tables and triggers
func (db *DB) initSchema() error {
	// Create main table for per-page storage
	mainTableQuery := `
		CREATE TABLE IF NOT EXISTS pdfs (
			path TEXT NOT NULL,
			page_num INTEGER NOT NULL,
			hash TEXT NOT NULL,
			content TEXT,
			last_scanned TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (path, page_num)
		);
		CREATE INDEX IF NOT EXISTS idx_pdfs_hash ON pdfs (hash);
		CREATE INDEX IF NOT EXISTS idx_pdfs_path ON pdfs (path);
	`

	if _, err := db.Exec(mainTableQuery); err != nil {
		return fmt.Errorf("creating pdfs table: %w", err)
	}

	// Create FTS table using helper
	if err := db.createFTSTable(db.DB); err != nil {
		return err
	}

	// Create triggers using helper
	if err := db.createTriggers(db.DB); err != nil {
		return err
	}

	return nil
}

// createFTSTable creates the FTS table using the provided executor.
func (db *DB) createFTSTable(exec executor) error {
	if db.verbose {
		log.Println("Ensuring FTS table pdfs_fts exists and is correctly configured...")
	}
	ftsTableQuery := `
		CREATE VIRTUAL TABLE IF NOT EXISTS pdfs_fts USING fts5(
			path UNINDEXED,
			page_num UNINDEXED,
			content_idx,
			tokenize = 'unicode61'
		);
	`
	if _, err := exec.Exec(ftsTableQuery); err != nil {
		return fmt.Errorf("creating/configuring pdfs_fts table: %w", err)
	}
	return nil
}

// createTriggers creates the FTS triggers using the provided executor.
func (db *DB) createTriggers(exec executor) error {
	if db.verbose {
		log.Println("Ensuring FTS triggers exist...")
	}
	triggers := []string{
		`
			CREATE TRIGGER IF NOT EXISTS pdfs_after_insert
			AFTER INSERT ON pdfs 
			BEGIN
				INSERT INTO pdfs_fts (path, page_num, content_idx) VALUES (new.path, new.page_num, new.content);
			END;
		`,
		`
			CREATE TRIGGER IF NOT EXISTS pdfs_after_delete
			AFTER DELETE ON pdfs
			BEGIN
				DELETE FROM pdfs_fts WHERE path = old.path AND page_num = old.page_num;
			END;
		`,
		`
			CREATE TRIGGER IF NOT EXISTS pdfs_after_update_content
			AFTER UPDATE OF content ON pdfs
			WHEN new.content IS NOT old.content
			BEGIN
				UPDATE pdfs_fts SET content_idx = new.content WHERE path = new.path AND page_num = new.page_num;
			END;
		`,
	}

	for _, triggerQuery := range triggers {
		if _, err := exec.Exec(triggerQuery); err != nil {
			// Attempt to extract trigger name for a more specific error message
			parts := strings.Fields(triggerQuery)
			triggerNameForError := "unknown trigger"
			if len(parts) > 5 && strings.ToUpper(parts[1]) == "TRIGGER" {
				triggerNameForError = parts[5]
			}
			return fmt.Errorf("creating trigger %s: %w", triggerNameForError, err)
		}
	}
	return nil
}

// GetStoredHash retrieves the stored hash for a PDF file (from any page)
func (db *DB) GetStoredHash(filePath string) (string, error) {
	var storedHash string
	err := db.QueryRow("SELECT hash FROM pdfs WHERE path = ? LIMIT 1", filePath).Scan(&storedHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No hash stored, treat as no error but empty string
		}
		return "", fmt.Errorf("querying stored hash for %s: %w", filePath, err)
	}
	return storedHash, nil
}

// UpsertPDFData inserts or updates PDF data in the database for all pages
func (db *DB) UpsertPDFData(filePath, hash string, pageContents []string) error {
	if db.verbose {
		log.Printf("Upserting PDF data for: %s (%d pages)", filePath, len(pageContents))
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for %s: %w", filePath, err)
	}
	defer tx.Rollback()

	// First, delete all existing pages for this file
	_, err = tx.Exec("DELETE FROM pdfs WHERE path = ?", filePath)
	if err != nil {
		return fmt.Errorf("deleting existing pages for %s: %w", filePath, err)
	}

	// Insert all pages
	stmt, err := tx.Prepare(`
		INSERT INTO pdfs (path, page_num, hash, content, last_scanned) 
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("preparing insert statement for %s: %w", filePath, err)
	}
	defer stmt.Close()

	for pageNum, content := range pageContents {
		_, err = stmt.Exec(filePath, pageNum+1, hash, content) // page numbers are 1-indexed
		if err != nil {
			return fmt.Errorf("inserting page %d for %s: %w", pageNum+1, filePath, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction for %s: %w", filePath, err)
	}

	return nil
}

// Search performs a full-text search and returns results
func (db *DB) Search(queryTerm string, limit int) (*sql.Rows, error) {
	return db.Query(
		`
			SELECT
				p.path,
				p.page_num,
				snippet(pdfs_fts, 2, '[HL]', '[/HL]', '...', 25) AS snippet,
				p.last_scanned
			FROM pdfs_fts
			JOIN pdfs AS p ON pdfs_fts.path = p.path AND pdfs_fts.page_num = p.page_num
			WHERE pdfs_fts MATCH ? ORDER BY p.path, p.page_num LIMIT ?;
		`,
		queryTerm, limit,
	)
}

// LiveSearch performs a search optimized for live/interactive results
func (db *DB) LiveSearch(queryTerm string, limit int) (*sql.Rows, error) {
	if queryTerm == "" {
		return nil, nil
	}

	return db.Query(
		`
			SELECT
				p.path,
				p.page_num,
				snippet(pdfs_fts, 2, '>>>', '<<<', ' ... ', 15) AS snippet
			FROM pdfs_fts
			JOIN pdfs AS p ON pdfs_fts.path = p.path AND pdfs_fts.page_num = p.page_num
			WHERE pdfs_fts MATCH ? ORDER BY rank LIMIT ?;
		`,
		queryTerm, limit,
	)
}

// RebuildFTS drops and recreates the FTS index
func (db *DB) RebuildFTS() error {
	if db.verbose {
		log.Println("Rebuilding Full-Text Search index...")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if commit is not successful

	// Drop triggers
	triggerNames := []string{"pdfs_after_insert", "pdfs_after_delete", "pdfs_after_update_content"}
	for _, triggerName := range triggerNames {
		if db.verbose {
			log.Printf("Dropping trigger %s if exists...", triggerName)
		}
		_, err := tx.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s;", triggerName))
		if err != nil {
			return fmt.Errorf("dropping trigger %s: %w", triggerName, err)
		}
	}

	// Drop FTS table
	if db.verbose {
		log.Println("Dropping FTS table pdfs_fts if exists...")
	}
	_, err = tx.Exec("DROP TABLE IF EXISTS pdfs_fts;")
	if err != nil {
		return fmt.Errorf("dropping pdfs_fts table: %w", err)
	}

	// Recreate FTS table using helper
	if err := db.createFTSTable(tx); err != nil {
		return err // Error already formatted by helper
	}

	// Recreate triggers using helper
	if err := db.createTriggers(tx); err != nil {
		return err // Error already formatted by helper
	}

	// Repopulate FTS table
	if db.verbose {
		log.Println("Repopulating FTS table from pdfs table...")
	}
	rows, err := tx.Query(`
		SELECT path, page_num, content FROM pdfs;
	`)
	if err != nil {
		return fmt.Errorf("querying pdfs table for repopulation: %w", err)
	}
	defer rows.Close()

	insertStmt, err := tx.Prepare(`
		INSERT INTO pdfs_fts (path, page_num, content_idx) VALUES (?, ?, ?);
	`)
	if err != nil {
		return fmt.Errorf("preparing FTS insert statement: %w", err)
	}
	defer insertStmt.Close()

	var repopulatedCount int
	for rows.Next() {
		var path, content string
		var pageNum int
		if err := rows.Scan(&path, &pageNum, &content); err != nil {
			log.Printf("Warning: Failed to scan row from pdfs table: %v. Skipping this row for FTS.", err)
			continue
		}
		_, err := insertStmt.Exec(path, pageNum, content)
		if err != nil {
			return fmt.Errorf("inserting into FTS table for %s page %d: %w", path, pageNum, err)
		}
		repopulatedCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing FTS rebuild transaction: %w", err)
	}

	if db.verbose {
		log.Printf("FTS rebuild completed successfully. Repopulated %d entries.", repopulatedCount)
	}

	return nil
}
