// internal/repository/db.go
package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // драйвер SQLite, регистрируется под именем "sqlite"
)

// OpenDB открывает базу данных и применяет все новые миграции из папки migrationsDir.
func OpenDB(dsn, migrationsDir string) (*sql.DB, error) {
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

	if err := runMigrations(db, migrationsDir); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// runMigrations выполняет по порядку все файлы *.sql из папки.
// Имена уже применённых миграций хранятся в таблице schema_migrations,
// поэтому каждый файл выполняется только один раз
// (а ALTER TABLE нельзя выполнять повторно).
func runMigrations(db *sql.DB, dir string) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)")
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files) // 001_..., 002_..., 003_...

	for _, file := range files {
		name := filepath.Base(file)

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			continue // эта миграция уже применялась
		}

		script, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		// скрипт и запись о нём - в одной транзакции: либо всё, либо ничего
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("миграция %s: %w", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}

		fmt.Println("применена миграция", name)
	}

	return nil
}
