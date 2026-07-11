// Package db содержит работу с Postgres подключение, миграции и UserStorage.
package db

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Connect is function for database connection.
// Повторяет подключение при временных сетевых ошибках (DNS, таймауты).
func Connect(databaseURL string, l *zap.Logger) (*sqlx.DB, error) {
	var db *sqlx.DB

	// Retry подключения к БД — при Docker DNS проблемах приходится ждать
	maxRetries := 20
	backoff := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var err error
		db, err = sqlx.Connect("postgres", databaseURL)
		if err == nil {
			break // успешно подключились
		}

		l.Warn(
			"ошибка подключения к БД, повторяем...",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
		)

		if attempt == maxRetries {
			l.Error(
				"failed db connection",
				zap.Error(err),
			)

			return nil, fmt.Errorf("failed database connection after %d attempts: %w", maxRetries, err)
		}

		time.Sleep(backoff)
		backoff *= 2 // 2s → 4s → 8s → 16s
	}

	// Настраиваем пул соединений.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Выполняем миграции из папки migrations/
	if err := runMigrations(databaseURL); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// runMigrations запускает миграции из папки migrations/
// Это автоматически применит все .up.sql файлы по порядку.
func runMigrations(databaseURL string) error {
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run up migrations: %w", err)
	}

	return nil
}

// func Connect(databaseURL string) (*sqlx.DB, error) {
// 	db, err := sqlx.Connect("postgres", databaseURL)
// 	if err != nil {
// 		return nil, err
// 	}
// 	db.SetMaxOpenConns(15)
// 	db.SetMaxIdleConns(5)
// 	return db, nil
// }
