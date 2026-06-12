package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type UserStorage struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewUserStorage(db *sqlx.DB, logger *zap.Logger) *UserStorage {
	return &UserStorage{
		db:     db,
		logger: logger,
	}
}

func (s *UserStorage) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info(
			"вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

func (s *UserStorage) CreateUser(userData domain.CreateUserDTO) (*domain.UserTG, error) {
	defer s.logDuration("CreateUser")()

	var user domain.UserTG

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
		s.logger.Error(
			"failed to create user",
			zap.String("user_id", user.ID),
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return &user, nil
}

func (s *UserStorage) GetAllUsers() ([]domain.UserTG, error) {
	defer s.logDuration("GetAllUser")()

	var users []domain.UserTG

	query := `
	SELECT id, trial, extra_devices_count, created_at
	FROM users
	ORDER BY created_at DESC
	`

	if err := s.db.Select(&users, query); err != nil {

		s.logger.Error("failed to get all users", zap.Error(err))

		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	return users, nil
}

func (s *UserStorage) GetUserByID(id string) (*domain.UserTG, error) {
	defer s.logDuration("GetUserByID")()

	var user domain.UserTG

	query := `
	SELECT id, trial, extra_devices_count, created_at
	FROM users
	WHERE id = $1
	`

	if err := s.db.Get(&user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Info("user not found", zap.String("id", id))

			return nil, domain.ErrUserNotFound
		}

		s.logger.Error(
			"failed to get user",
			zap.String("id", id),
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to get user: %w", err)

	}

	return &user, nil
}

func (s *UserStorage) UpdateUser(
	id string,
	updateData domain.UpdateUserTGDTO,
) (*domain.UserTG, error) {
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

	var updatedUser domain.UserTG
	if err := s.db.QueryRowx(
		query,
		user.Trial,
		user.ExtraDevicesCount,
		id,
	).StructScan(&updatedUser); err != nil {

		s.logger.Error(
			"failed to update user",
			zap.Any("updateData", updateData),
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &updatedUser, nil
}

func (s *UserStorage) GetActiveUserIDs() ([]string, error) {
	var userIDs []string
	query := `SELECT id FROM users`
	if err := s.db.Select(&userIDs, query); err != nil {
		s.logger.Error(
			"failed to get all user IDs",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to get all user IDs: %w", err)
	}

	return userIDs, nil
}
