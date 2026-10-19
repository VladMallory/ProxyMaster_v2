package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	// Для init().
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open открывает пул к Postgres и проверяет соединение.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, errors.New("postgres: пустой DATABASE_URL")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		// %w сохраняет цепочку для errors.Is/As.
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	// Модель «каждое устройство отдельно»: одна строка devices = одно
	// оплаченное место со своим сроком. hwid пуст, пока устройство не подключилось.
	const devices = `
CREATE TABLE IF NOT EXISTS devices (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username   TEXT        NOT NULL,
    hwid       TEXT,
    label      TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`
	// Уникальность (username, hwid) только для привязанных мест: свободных
	// строк с hwid IS NULL может быть сколько угодно, поэтому индекс частичный.
	const devicesUsernameHwid = `
CREATE UNIQUE INDEX IF NOT EXISTS devices_username_hwid_uq
    ON devices (username, hwid) WHERE hwid IS NOT NULL`
	const devicesUsername = `
CREATE INDEX IF NOT EXISTS devices_username_idx ON devices (username)`
	const devicesToRevoke = `
CREATE INDEX IF NOT EXISTS devices_to_revoke_idx
    ON devices (expires_at) WHERE revoked_at IS NULL`
	// Дедупликация оплат: одна строка на платёж, вставляется В ТРАНЗАКЦИИ покупки,
	// поэтому повторный вебхук не создаст места дважды.
	const processedPayments = `
CREATE TABLE IF NOT EXISTS processed_payments (
    payment_id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

	// Выполняем в том же ctx из main отмена старта прервёт и миграцию.
	statements := []struct {
		name string
		sql  string
	}{
		{"devices", devices},
		{"devices_username_hwid_uq", devicesUsernameHwid},
		{"devices_username_idx", devicesUsername},
		{"devices_to_revoke_idx", devicesToRevoke},
		{"processed_payments", processedPayments},
	}

	for _, s := range statements {
		if _, err := db.ExecContext(ctx, s.sql); err != nil {
			return fmt.Errorf("postgres migrate %s: %w", s.name, err)
		}
	}

	return nil
}
