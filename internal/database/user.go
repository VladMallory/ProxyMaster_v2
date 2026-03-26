// Package database предоставляет UserStorage для взаимодействия с таблицей
// пользователей в базе данных. Он включает методы для создания, получения,
// обновления и управления данными пользователей, а также другие операции,
// связанные с пользователями.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
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

// CreateUser создает пользователя в DB.
func (s *UserStorage) CreateUser(userData models.CreateUserDTO) (*models.UserTG, error) {
	defer s.logDuration("CreateUser")()

	var user models.UserTG

	query := `
	INSERT INTO users (id, trial, extra_devices_count, created_at)
	VALUES ($1, $2, $3, $4)
	RETURNING id, trial, extra_devices_count, created_at
	`

	now := time.Now()
	err := s.db.QueryRowx(
		query,
		userData.ID,
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
	SELECT id, trial, extra_devices_count, created_at
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
	SELECT id, trial, extra_devices_count, created_at
	FROM users
	WHERE id = $1
	`

	if err := s.db.Get(&user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Info(
				"user not found",
				logger.Field{Key: "id", Value: id},
			)

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

	if updateData.Trial != nil {
		user.Trial = *updateData.Trial
	}

	if updateData.ExtraDevicesCount != nil {
		user.ExtraDevicesCount = *updateData.ExtraDevicesCount
	}

	query := `
	UPDATE users
	SET trial = $1,
	    extra_devices_count = $2
	WHERE id = $3
	RETURNING id, trial, extra_devices_count, created_at
	`

	var updatedUser models.UserTG
	if err := s.db.QueryRowx(
		query,
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

// GetActiveUserIDs возвращает ID всех активных пользователей.
// Этот метод используется для получения списка всех пользователей, которым будет отправлена рассылка.
func (s *UserStorage) GetActiveUserIDs() ([]string, error) {
	var userIDs []string
	query := `SELECT id FROM users`
	if err := s.db.Select(&userIDs, query); err != nil {
		s.logger.Error("failed to get all user IDs", logger.Field{Key: "err_msg", Value: err})
		return nil, fmt.Errorf("failed to get all user IDs: %w", err)
	}
	return userIDs, nil
}
