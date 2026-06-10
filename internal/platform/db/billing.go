//nolint:cyclop,nlreturn
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	subsDevice "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/device"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

func (s *UserStorage) PrepayDeviceAddonsAtomic(
	userID string,
	priceRUB int,
	chargePeriod time.Duration,
) (count int, err error) {
	defer s.logDuration("PrepayDeviceAddonsAtomic")()
	if priceRUB <= 0 {
		return 0, errors.New("priceRUB должен быть > 0")
	}

	if chargePeriod <= 0 {
		return 0, errors.New("chargePeriod должен быть > 0")
	}

	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	lockQuery := `SELECT id FROM users WHERE id = $1 FOR UPDATE`
	var lockedID string
	if err := tx.QueryRowx(lockQuery, userID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error(
				"user not found when locking for PrepayDeviceAddonsAtomic",
				zap.String("user_id", userID),
				zap.Error(domain.ErrUserNotFound),
			)
			return 0, domain.ErrUserNotFound
		}
		s.logger.Error(
			"failed to lock user for PrepayDeviceAddonsAtomic",
			zap.String("user_id", userID),
			zap.Error(err),
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
		s.logger.Error(
			"failed to select active addons for PrepayDeviceAddonsAtomic",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to select active addons: %w", err)
	}
	if len(addonIDs) == 0 {
		if err := tx.Commit(); err != nil {
			s.logger.Error(
				"failed to commit PrepayDeviceAddonsAtomic with no addons",
				zap.Error(err),
			)
			return 0, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return 0, subsDevice.ErrNoActiveDeviceAddons
	}

	seconds := int64(chargePeriod.Seconds())
	updateQuery := `
	UPDATE device_addons
	SET next_charge_at = next_charge_at + ($1 * INTERVAL '1 second')
	WHERE id = ANY($2)
	`
	if _, err := tx.Exec(updateQuery, seconds, pq.Array(addonIDs)); err != nil {
		s.logger.Error(
			"failed to update next_charge_at for PrepayDeviceAddonsAtomic",
			zap.Strings("addon_ids", addonIDs),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to update next_charge_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error(
			"failed to commit PrepayDeviceAddonsAtomic transaction",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return len(addonIDs), nil
}

type dueDeviceAddonRow struct {
	ID     string `db:"id"`
	UserID string `db:"user_id"`
}

func (s *UserStorage) ProcessDueDeviceAddonsBilling(
	now time.Time,
	limit int,
	priceRUB int,
	chargePeriod time.Duration,
) (map[string]int, error) {
	defer s.logDuration("ProcessDueDeviceAddonsBilling")()
	if limit <= 0 {
		limit = 200
	}
	if priceRUB <= 0 {
		return nil, errors.New("priceRUB должен быть > 0")
	}
	if chargePeriod <= 0 {
		return nil, errors.New("chargePeriod должен быть > 0")
	}

	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		s.logger.Error(
			"failed to begin transaction",
			zap.Error(err),
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
		s.logger.Error(
			"failed to select due device addons",
			zap.Int("limit", limit),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to select due device addons: %w", err)
	}
	if len(due) == 0 {
		if err := tx.Commit(); err != nil {
			s.logger.Error(
				"failed to commit empty billing transaction",
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to commit empty billing transaction: %w", err)
		}
		return make(map[string]int), nil
	}

	byUser := make(map[string][]string)
	for _, row := range due {
		byUser[row.UserID] = append(byUser[row.UserID], row.ID)
	}

	result := make(map[string]int, len(byUser))
	for userID, addonIDs := range byUser {
		if len(addonIDs) == 0 {
			continue
		}
		if err := s.deactivateDeviceAddonsTx(tx, addonIDs); err != nil {
			s.logger.Error(
				"failed to deactivate device addons",
				zap.String("user_id", userID),
				zap.Strings("addon_ids", addonIDs),
				zap.Error(err),
			)
			return nil, err
		}
		if err := s.decrementExtraDevicesCountTx(tx, userID, len(addonIDs)); err != nil {
			s.logger.Error(
				"failed to decrement extra_devices_count",
				zap.String("user_id", userID),
				zap.Int("amount", len(addonIDs)),
				zap.Error(err),
			)
			return nil, err
		}
		countQuery := `
		SELECT COUNT(*)
		FROM (
			SELECT id
			FROM device_addons
			WHERE user_id = $1 AND active = TRUE
			FOR UPDATE
		) AS active_addons
		`
		var activeCount int
		if err := tx.QueryRowx(countQuery, userID).Scan(&activeCount); err != nil {
			s.logger.Error(
				"failed to count remaining active addons",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to count remaining active addons: %w", err)
		}
		result[userID] = activeCount
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error(
			"failed to commit billing transaction",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to commit billing transaction: %w", err)
	}

	return result, nil
}
