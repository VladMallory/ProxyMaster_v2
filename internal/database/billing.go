package database

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/pkg/logger"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// PrepayDeviceAddonsAtomic оплачивает все активные дополнительные устройства пользователя.
func (s *UserStorage) PrepayDeviceAddonsAtomic(
	userID string,
	priceRUB int,
	chargePeriod time.Duration,
) (count int, err error) {
	defer s.logDuration("PrepayDeviceAddonsAtomic")()
	if priceRUB <= 0 {
		err := fmt.Errorf("priceRUB должен быть > 0")
		s.logger.Error("invalid priceRUB",
			logger.Field{Key: "priceRUB", Value: priceRUB},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, err
	}
	if chargePeriod <= 0 {
		err := fmt.Errorf("chargePeriod должен быть > 0")
		s.logger.Error("invalid chargePeriod",
			logger.Field{Key: "chargePeriod", Value: chargePeriod},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, err
	}

	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		s.logger.Error("failed to begin transaction for PrepayDeviceAddonsAtomic",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	lockQuery := `SELECT id FROM users WHERE id = $1 FOR UPDATE`
	var lockedID string
	if err := tx.QueryRowx(lockQuery, userID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("user not found when locking for PrepayDeviceAddonsAtomic",
				logger.Field{Key: "user_id", Value: userID},
				logger.Field{Key: "err_msg", Value: domain.ErrUserNotFound},
			)
			return 0, domain.ErrUserNotFound
		}
		s.logger.Error("failed to lock user for PrepayDeviceAddonsAtomic",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, fmt.Errorf("failed to lock user: %w", err)
	}

	addonsQuery := `
	SELECT id
	FROM device_addons
	WHERE user_id = $1 AND active = TRUE
	FOR UPDATE
	`
	var addonIDs []string
	if err := tx.Select(&addonIDs, addonsQuery, userID); err != nil {
		s.logger.Error("failed to select active addons for PrepayDeviceAddonsAtomic",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, fmt.Errorf("failed to select active addons: %w", err)
	}
	if len(addonIDs) == 0 {
		if err := tx.Commit(); err != nil {
			s.logger.Error("failed to commit PrepayDeviceAddonsAtomic with no addons",
				logger.Field{Key: "err_msg", Value: err},
			)
			return 0, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return 0, domain.ErrNoActiveDeviceAddons
	}

	seconds := int64(chargePeriod.Seconds())
	updateQuery := `
	UPDATE device_addons
	SET next_charge_at = next_charge_at + ($1 * INTERVAL '1 second')
	WHERE id = ANY($2)
	`
	if _, err := tx.Exec(updateQuery, seconds, pq.Array(addonIDs)); err != nil {
		s.logger.Error("failed to update next_charge_at for PrepayDeviceAddonsAtomic",
			logger.Field{Key: "addon_ids", Value: addonIDs},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, fmt.Errorf("failed to update next_charge_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit PrepayDeviceAddonsAtomic transaction",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return len(addonIDs), nil
}

type dueDeviceAddonRow struct {
	ID     string `db:"id"`
	UserID string `db:"user_id"`
}

// ProcessDueDeviceAddonsBilling обрабатывает биллинг просроченных дополнительных устройств.
// Находит истекшие device_addons и деактивирует их, обновляя счетчик users.extra_devices_count.
func (s *UserStorage) ProcessDueDeviceAddonsBilling(
	now time.Time,
	limit int,
	priceRUB int,
	chargePeriod time.Duration,
) ([]string, error) {
	defer s.logDuration("ProcessDueDeviceAddonsBilling")()
	if limit <= 0 {
		limit = 200
	}
	if priceRUB <= 0 {
		err := fmt.Errorf("priceRUB должен быть > 0")
		s.logger.Error("invalid priceRUB",
			logger.Field{Key: "priceRUB", Value: priceRUB},
			logger.Field{Key: "err_msg", Value: err},
		)
		return nil, err
	}
	if chargePeriod <= 0 {
		err := fmt.Errorf("chargePeriod должен быть > 0")
		s.logger.Error("invalid chargePeriod",
			logger.Field{Key: "chargePeriod", Value: chargePeriod},
			logger.Field{Key: "err_msg", Value: err},
		)
		return nil, err
	}

	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		s.logger.Error("failed to begin transaction",
			logger.Field{Key: "err_msg", Value: err},
		)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selectDueQuery := `
	SELECT id, user_id
	FROM device_addons
	WHERE active = TRUE AND next_charge_at <= $1
	ORDER BY user_id, next_charge_at ASC
	LIMIT $2
	FOR UPDATE
	`

	var due []dueDeviceAddonRow
	if err := tx.Select(&due, selectDueQuery, now, limit); err != nil {
		s.logger.Error("failed to select due device addons",
			logger.Field{Key: "limit", Value: limit},
			logger.Field{Key: "err_msg", Value: err},
		)
		return nil, fmt.Errorf("failed to select due device addons: %w", err)
	}
	if len(due) == 0 {
		if err := tx.Commit(); err != nil {
			s.logger.Error("failed to commit empty billing transaction",
				logger.Field{Key: "err_msg", Value: err},
			)
			return nil, fmt.Errorf("failed to commit empty billing transaction: %w", err)
		}
		return nil, nil
	}

	byUser := make(map[string][]string)
	for _, row := range due {
		byUser[row.UserID] = append(byUser[row.UserID], row.ID)
	}

	usersToReset := make([]string, 0, len(byUser))
	for userID, addonIDs := range byUser {
		if len(addonIDs) == 0 {
			continue
		}
		// Деактивируем истекшие устройства.
		if err := s.deactivateDeviceAddonsTx(tx, addonIDs); err != nil {
			s.logger.Error("failed to deactivate device addons",
				logger.Field{Key: "user_id", Value: userID},
				logger.Field{Key: "addon_ids", Value: addonIDs},
				logger.Field{Key: "err_msg", Value: err},
			)
			return nil, err
		}
		// Обновляем счетчик доп. устройств.
		if err := s.decrementExtraDevicesCountTx(tx, userID, len(addonIDs)); err != nil {
			s.logger.Error("failed to decrement extra_devices_count",
				logger.Field{Key: "user_id", Value: userID},
				logger.Field{Key: "amount", Value: len(addonIDs)},
				logger.Field{Key: "err_msg", Value: err},
			)
			return nil, err
		}
		usersToReset = append(usersToReset, userID)
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit billing transaction",
			logger.Field{Key: "err_msg", Value: err},
		)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return usersToReset, nil
}
