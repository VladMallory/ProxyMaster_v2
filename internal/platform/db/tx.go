package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

func (s *UserStorage) setExtraDevicesCountTx(tx *sqlx.Tx, userID string, cnt int) error {
	defer s.logDuration("setExtraDevicesCountTx")()
	query := `
	UPDATE users
	SET extra_devices_count = $1
	WHERE id = $2
	`
	if _, err := tx.Exec(query, cnt, userID); err != nil {
		s.logger.Error(
			"failed to update extra_devices_count in tx",
			zap.String("user_id", userID),
			zap.Int("cnt", cnt),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update extra_devices_count: %w", err)
	}
	return nil
}

func (s *UserStorage) deactivateDeviceAddonsTx(tx *sqlx.Tx, addonIDs []string) error {
	defer s.logDuration("deactivateDeviceAddonsTx")()
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE id = ANY($1)
	`
	if _, err := tx.Exec(query, pq.Array(addonIDs)); err != nil {
		s.logger.Error(
			"failed to deactivate device addons in tx",
			zap.Strings("addon_ids", addonIDs),
			zap.Error(err),
		)
		return fmt.Errorf("failed to deactivate device addons: %w", err)
	}
	return nil
}

func (s *UserStorage) decrementExtraDevicesCountTx(tx *sqlx.Tx, userID string, amount int) error {
	defer s.logDuration("decrementExtraDevicesCountTx")()
	query := `
	UPDATE users
	SET extra_devices_count = GREATEST(extra_devices_count - $1, 0)
	WHERE id = $2
	`
	if _, err := tx.Exec(query, amount, userID); err != nil {
		s.logger.Error(
			"failed to decrement extra_devices_count in tx",
			zap.String("user_id", userID),
			zap.Int("amount", amount),
			zap.Error(err),
		)
		return fmt.Errorf("failed to decrement extra_devices_count: %w", err)
	}
	return nil
}
