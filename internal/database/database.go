// Package database содержит работу с Postgres подключение и простые миграции схемы.
package database

import (
	"ProxyMaster_v2/pkg/logger"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Connect is function for database connection
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

	// Делаем мягкую миграцию схемы.
	// Это нужно, потому что init.sql выполняется только при первом старте контейнера с пустым volume.
	if err := ensureSchema(db); err != nil {
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
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

func ensureSchema(db *sqlx.DB) error {
	// Добавляем колонку extra_devices_count, если её еще нет.
	// Она нужна для отображения в личном кабинете.
	if _, err := db.ExecContext(context.Background(), `
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS extra_devices_count INTEGER NOT NULL DEFAULT 0
	`); err != nil {
		return fmt.Errorf("failed to add extra_devices_count: %w", err)
	}

	// Создаем таблицу услуг дополнительных устройств, если её еще нет.
	// Каждая покупка = отдельная строка с собственной датой следующего списания.
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS device_addons (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(20) NOT NULL,
			next_charge_at TIMESTAMP NOT NULL,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_device_addons_user
				FOREIGN KEY (user_id)
				REFERENCES users(id)
				ON DELETE CASCADE
				ON UPDATE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("failed to create device_addons: %w", err)
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
