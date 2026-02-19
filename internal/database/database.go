// Package database содержит работу с Postgres подключение и простые миграции схемы.
package database

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Connect is function for database connection.
func Connect(databaseURL string, l logger.Logger) (*sqlx.DB, error) {
	// Подключаемся к Postgres.
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		l.Error("failed db connection",
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, fmt.Errorf("failed database connection: %w", err)
	}

	// Настраиваем пул соединений.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := ensurePostgresRole(db, databaseURL); err != nil {
		return nil, fmt.Errorf("failed to ensure postgres role: %w", err)
	}

	// Выполняем миграции из папки migrations/
	if err := runMigrations(databaseURL); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func ensurePostgresRole(db *sqlx.DB, databaseURL string) error {
	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("invalid database url: %w", err)
	}

	userInfo := parsedURL.User
	if userInfo == nil {
		return fmt.Errorf("database url missing user info")
	}

	password, hasPassword := userInfo.Password()
	if !hasPassword || strings.TrimSpace(password) == "" {
		return fmt.Errorf("database url missing password")
	}

	var exists bool
	if err := db.Get(&exists, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'postgres')`); err != nil {
		return fmt.Errorf("failed to check postgres role: %w", err)
	}
	if exists {
		return nil
	}

	createRoleQuery := fmt.Sprintf(
		"CREATE ROLE postgres WITH LOGIN PASSWORD %s SUPERUSER",
		pq.QuoteLiteral(password),
	)
	if _, err := db.Exec(createRoleQuery); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create postgres role: %w", err)
	}

	return nil
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

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
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
