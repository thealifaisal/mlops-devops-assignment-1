package db

import (
	"database/sql"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"os"
)

// Migrate executes SQL files found under the given directory in lexical order.
// Each .sql file is executed as a single statement batch.
func Migrate(db *sql.DB, migrationsDir string) error {
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
	// execute in order
	for _, f := range files {
		content, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(content)); err != nil {
			log.Printf("migration failed %s: %v", f, err)
			return err
		}
	}
	return nil
}
