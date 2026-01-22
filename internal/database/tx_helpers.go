package database

import (
	"ProxyMaster_v2/pkg/logger"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// deactivateAllDeviceAddonsTx отключает все доп. устройства пользователя в рамках транзакции.
func (s *UserStorage) deactivateAllDeviceAddonsTx(tx *sqlx.Tx, userID string) error {
	defer s.logDuration("deactivateAllDeviceAddonsTx")()
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE user_id = $1 AND active = TRUE
	`
	if _, err := tx.Exec(query, userID); err != nil {
		s.logger.Error("failed to deactivate device addons in tx",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to deactivate device addons: %w", err)
	}
	return nil
}

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
