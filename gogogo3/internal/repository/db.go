// internal/repository/db.go
package repository

import (
	"database/sql"
	"os"

	_ "modernc.org/sqlite" // драйвер SQLite, регистрируется под именем "sqlite"
)

// OpenDB открывает файл базы данных и выполняет SQL-скрипт миграции.
func OpenDB(dsn, migrationFile string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// SQLite пишет в файл, поэтому одного соединения достаточно
	// (так не бывает ошибок "database is locked").
	db.SetMaxOpenConns(1)

	// sql.Open только готовит соединение, Ping проверяет, что база реально открылась
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	script, err := os.ReadFile(migrationFile)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(string(script)); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
