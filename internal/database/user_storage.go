// Package database предоставляет UserStorage для взаимодействия с таблицей
// пользователей в базе данных. Он включает методы для создания, получения,
// обновления и управления данными пользователей.
package database

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// UserStorage structure for working with users table
type UserStorage struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewUserStorage is constructor for UserStorage struct
func NewUserStorage(db *sqlx.DB, logger logger.Logger) *UserStorage {
	return &UserStorage{
		db:     db,
		logger: logger,
	}
}

// logDuration логирует время выполнения метода.
func (s *UserStorage) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info("вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

// CreateUser создает пользователя в DB
func (s *UserStorage) CreateUser(userData models.CreateUserTGDTO) (*models.UserTG, error) {
	defer s.logDuration("CreateUser")()

	var user models.UserTG

	query := `
	INSERT INTO users (id, balance, trial, extra_devices_count, created_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, balance, trial, extra_devices_count, created_at
	`

	now := time.Now()
	err := s.db.QueryRowx(
		query,
		userData.ID,
		userData.Balance,
		userData.Trial,
		0,
		now,
	).StructScan(&user)

	if err != nil {
		s.logger.Error("failed to create user",
			logger.Field{Key: "user_id", Value: user.ID},
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return &user, nil
}

// GetAllUsers is method for getting all users
func (s *UserStorage) GetAllUsers() ([]models.UserTG, error) {
	defer s.logDuration("GetAllUser")()

	var users []models.UserTG

	query := `
	SELECT id, balance, trial, extra_devices_count, created_at
	FROM users
	ORDER BY created_at DESC
	`

	if err := s.db.Select(&users, query); err != nil {

		s.logger.Error("failed to get all users",
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	return users, nil
}

// GetUserByID is methos for getting user by id
func (s *UserStorage) GetUserByID(id string) (*models.UserTG, error) {
	defer s.logDuration("GetUserByID")()

	var user models.UserTG

	query := `
	SELECT id, balance, trial, extra_devices_count, created_at
	FROM users
	WHERE id = $1
	`

	if err := s.db.Get(&user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error(
				"user not found",
				logger.Field{Key: "id", Value: id},
				logger.Field{Key: "err_msg", Value: err},
			)

			// Возвращем ошибку о том что пользователя нет в DB
			return nil, domain.ErrUserNotFound
		}

		s.logger.Error("failed to get user",
			logger.Field{Key: "id", Value: id},
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, fmt.Errorf("failed to get user: %w", err)

	}

	return &user, nil
}

// UpdateUser обновляет юзера.
func (s *UserStorage) UpdateUser(
	id string,
	updateData models.UpdateUserTGDTO,
) (*models.UserTG, error) {
	defer s.logDuration("UpdateUser")()
	user, err := s.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	if updateData.Balance != nil {
		user.Balance = *updateData.Balance
	}

	if updateData.Trial != nil {
		user.Trial = *updateData.Trial
	}

	if updateData.ExtraDevicesCount != nil {
		user.ExtraDevicesCount = *updateData.ExtraDevicesCount
	}

	query := `
	UPDATE users
	SET balance = $1,
	    trial = $2,
	    extra_devices_count = $3
	WHERE id = $4
	RETURNING id, balance, trial, extra_devices_count, created_at
	`

	var updatedUser models.UserTG
	if err := s.db.QueryRowx(
		query,
		user.Balance,
		user.Trial,
		user.ExtraDevicesCount,
		id,
	).StructScan(&updatedUser); err != nil {

		s.logger.Error("failed to update user",
			logger.Field{Key: "updateData", Value: updateData},
			logger.Field{Key: "err_msg", Value: err},
		)

		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &updatedUser, nil
}

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

// GetActiveUserIDs возвращает ID всех активных пользователей.
// Этот метод используется для получения списка всех пользователей, которым будет отправлена рассылка.
func (s *UserStorage) GetActiveUserIDs() ([]string, error) {
	defer s.logDuration("GetActiveUserIDs")()
	var userIDs []string
	query := `SELECT id FROM users`
	if err := s.db.Select(&userIDs, query); err != nil {
		s.logger.Error("failed to get all user IDs", logger.Field{Key: "err_msg", Value: err})
		return nil, fmt.Errorf("failed to get all user IDs: %w", err)
	}
	return userIDs, nil
}
