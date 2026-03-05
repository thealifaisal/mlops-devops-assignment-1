package db

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
)

// Connect opens a Postgres connection using the provided DSN.
func Connect(dsn string) (*sql.DB, error) {
    if dsn == "" {
        return nil, fmt.Errorf("empty DSN")
    }
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, err
    }
    return db, nil
}
