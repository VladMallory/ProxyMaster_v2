// Package database for working with database
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ProxyMaster_v2/internal/models"

	"github.com/jmoiron/sqlx"
)

// UserStorage structure for working with users table
type UserStorage struct {
	db *sqlx.DB
}

// NewUserStorage is constructor for UserStorage struct
func NewUserStorage(db *sqlx.DB) *UserStorage {
	return &UserStorage{
		db: db,
	}
}

// CreateUser создает пользователя в DB
func (s *UserStorage) CreateUser(userData models.CreateUserTGDTO) (*models.UserTG, error) {
	var user models.UserTG

	query := `
	INSERT INTO users (id, balance, trial, created_at)
	VALUES ($1, $2, $3, $4)
	RETURNING id, balance, trial, created_at
	`

	now := time.Now()
	err := s.db.QueryRowx(
		query,
		userData.ID,
		userData.Balance,
		userData.Trial,
		now,
	).StructScan(&user)
	if err != nil {
		slog.Error(
			"failed to create user",
			"error_message", err,
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return &user, nil
}

// GetAllUsers is method for getting all users
func (s *UserStorage) GetAllUsers() ([]models.UserTG, error) {
	var users []models.UserTG

	query := `
	SELECT *
	FROM users
	ORDER BY created_at DESC
	`

	if err := s.db.Select(&users, query); err != nil {
		slog.Error(
			"failed to get users",
			"error_message", err,
		)

		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	return users, nil
}

// GetUserByID is methos for getting user by id
func (s *UserStorage) GetUserByID(id string) (*models.UserTG, error) {
	var user models.UserTG
	query := `
	SELECT id, balance, trial, created_at
	FROM users
	WHERE id = $1
	`

	if err := s.db.Get(&user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Error(
				"user not found",
				"id", id,
				"error_message", err,
			)

			return nil, fmt.Errorf("failed to get user by id: %w", err)
		}
		slog.Error(
			"failed to get user",
			"id", id,
			"error_message", err,
		)
	}

	return &user, nil
}

// UpdateUser обновляет юзера.
func (s *UserStorage) UpdateUser(id string, updateData models.UpdateUserTGDTO) (*models.UserTG, error) {
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

	query := `
	UPDATE users
	SET balance = $1, trial = $2
	WHERE id = $3
	RETURNING id, balance, trial, created_at
	`

	var updatedUser models.UserTG
	if err := s.db.QueryRowx(
		query,
		user.Balance,
		user.Trial,
		id,
	).StructScan(&updatedUser); err != nil {
		slog.Error(
			"failed to update user",
			"updateData", updateData,
			"error_message", err,
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return &updatedUser, nil
}
