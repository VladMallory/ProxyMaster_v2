package database

import (
	"ProxyMaster_v2/pkg/logger"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// TryDebitBalance пытается списать amount с баланса пользователя атомарно.
// Если денег не хватает, ok=false и баланс не меняется.
func (s *UserStorage) TryDebitBalance(
	userID string,
	amount int,
) (newBalance int, ok bool, err error) {
	defer s.logDuration("TryDebitBalance")()
	if amount <= 0 {
		err := fmt.Errorf("amount должен быть > 0")
		s.logger.Error("invalid amount",
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
	if err := s.db.QueryRowx(query, amount, userID).Scan(&bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		s.logger.Error("failed to debit balance",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "amount", Value: amount},
			logger.Field{Key: "err_msg", Value: err},
		)
		return 0, false, err
	}

	return bal, true, nil
}

// tryDebitBalanceTx пытается списать сумму в рамках транзакции.
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
