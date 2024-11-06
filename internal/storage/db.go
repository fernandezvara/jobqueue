package storage

import (
	"database/sql"
	"fmt"
	"time"
)

func NewDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %w", err)
	}

	// Configurar el pool de conexiones
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("error initializing schema: %w", err)
	}
	return nil
}
