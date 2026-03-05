package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate executes SQL files found under the given directory in lexical order.
// Each .sql file is executed as a single statement batch. Applied files are
// recorded in `schema_migrations` to make the runner idempotent.
func Migrate(db *sql.DB, migrationsDir string) error {
	// ensure migrations table exists
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMP DEFAULT now())`); err != nil {
		return err
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}
	// collect .sql files
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(files)

	for _, f := range files {
		name := filepath.Base(f)
		var existing string
		err := db.QueryRow("SELECT filename FROM schema_migrations WHERE filename=$1", name).Scan(&existing)
		if err == nil {
			// already applied
			log.Printf("skipping applied migration %s", name)
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}

		content, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			log.Printf("migration failed %s: %v", f, err)
			return err
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (filename) VALUES ($1)", name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
