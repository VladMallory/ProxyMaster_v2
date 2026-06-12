// Package db содержит работу с Postgres подключение, миграции и UserStorage.
package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Connect is function for database connection.
func Connect(databaseURL string, l *zap.Logger) (*sqlx.DB, error) {
	// Подключаемся к Postgres.
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		l.Error(
			"failed db connection",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed database connection: %w", err)
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
