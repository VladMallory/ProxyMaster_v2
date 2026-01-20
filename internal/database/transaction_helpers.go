package database

import (
	"ProxyMaster_v2/pkg/logger"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// tryDebitBalanceTx пытается списать amount с баланса пользователя в рамках транзакции.
func (s *UserStorage) tryDebitBalanceTx(
	tx *sqlx.Tx,
	userID string,
	amount int,
) (newBalance int, ok bool, err error) {
	defer s.logDuration("tryDebitBalanceTx")()
	if amount <= 0 {
		err := fmt.Errorf("amount должен быть > 0")
		s.logger.Error("invalid amount in tryDebitBalanceTx",
			logger.Field{Key: "amount", Value: amount},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, false, err
	}

	query := `
	UPDATE users
	SET balance = balance - $1
	WHERE id = $2 AND balance >= $1
	RETURNING balance
	`

	var bal int
	if err := tx.QueryRowx(query, amount, userID).Scan(&bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		s.logger.Error("failed to debit balance in tryDebitBalanceTx",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "amount", Value: amount},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, false, fmt.Errorf("failed to debit balance: %w", err)
	}

	return bal, true, nil
}

// deactivateAllDeviceAddonsTx отключает все доп. устройства в рамках транзакции.
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

// setExtraDevicesCountTx обновляет счётчик дополнительных устройств в рамках транзакции.
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

// updateDeviceAddonsNextChargeAtTx обновляет дату следующего списания для набора addons в рамках транзакции.
func (s *UserStorage) updateDeviceAddonsNextChargeAtTx(
	tx *sqlx.Tx,
	addonIDs []string,
	nextChargeAt time.Time,
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
