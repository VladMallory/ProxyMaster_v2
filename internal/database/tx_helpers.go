package database

import (
	"ProxyMaster_v2/pkg/logger"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// setExtraDevicesCountTx устанавливает количество доп. устройств в рамках транзакции.
func (s *UserStorage) setExtraDevicesCountTx(tx *sqlx.Tx, userID string, cnt int) error {
	defer s.logDuration("setExtraDevicesCountTx")()
	query := `
	UPDATE users
	SET extra_devices_count = $1
	WHERE id = $2
	`
	if _, err := tx.Exec(query, cnt, userID); err != nil {
		s.logger.Error("failed to update extra_devices_count in tx",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "cnt", Value: cnt},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to update extra_devices_count: %w", err)
	}
	return nil
}

// updateDeviceAddonsNextChargeAtTx обновляет время следующего списания для доп. устройств в транзакции.
func (s *UserStorage) updateDeviceAddonsNextChargeAtTx(
	tx *sqlx.Tx,
	addonIDs []string,
	nextChargeAt interface{},
) error {
	defer s.logDuration("updateDeviceAddonsNextChargeAtTx")()
	query := `
	UPDATE device_addons
	SET next_charge_at = $1
	WHERE id = ANY($2)
	`
	if _, err := tx.Exec(query, nextChargeAt, pq.Array(addonIDs)); err != nil {
		s.logger.Error("failed to update next_charge_at in tx",
			logger.Field{Key: "addon_ids", Value: addonIDs},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to update next_charge_at: %w", err)
	}
	return nil
}

// deactivateDeviceAddonsTx деактивирует доп. устройства в транзакции.
func (s *UserStorage) deactivateDeviceAddonsTx(tx *sqlx.Tx, addonIDs []string) error {
	defer s.logDuration("deactivateDeviceAddonsTx")()
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE id = ANY($1)
	`
	if _, err := tx.Exec(query, pq.Array(addonIDs)); err != nil {
		s.logger.Error("failed to deactivate device addons in tx",
			logger.Field{Key: "addon_ids", Value: addonIDs},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to deactivate device addons: %w", err)
	}
	return nil
}

// decrementExtraDevicesCountTx уменьшает количество доп. устройств в транзакции.
func (s *UserStorage) decrementExtraDevicesCountTx(tx *sqlx.Tx, userID string, amount int) error {
	defer s.logDuration("decrementExtraDevicesCountTx")()
	query := `
	UPDATE users
	SET extra_devices_count = GREATEST(extra_devices_count - $1, 0)
	WHERE id = $2
	`
	if _, err := tx.Exec(query, amount, userID); err != nil {
		s.logger.Error("failed to decrement extra_devices_count in tx",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "amount", Value: amount},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to decrement extra_devices_count: %w", err)
	}
	return nil
}
