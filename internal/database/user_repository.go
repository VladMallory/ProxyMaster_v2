// Package database for working with database
package database

import (
	"ProxyMaster_v2/internal/models"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

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

// CreateUser is method for creating user
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

// UpdateUser обновляет юзера, можно че кайф обновить(пока только баланс или триал), логику того,
// что обновляем будет писать в хэндлере бота
func (s *UserStorage) UpdateUser(id string, updateData models.UpdateUserTGDTO) (*models.UserTG, error) {
	var updatedUser *models.UserTG

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
	SET balance = $1, trial = $2, created_at = $3
	WHERE id = &4
	RETURNING id, balance, trial, created_at
	`

	if err := s.db.QueryRowx(
		query,
		user.Balance,
		user.Trial,
		id,
	).StructScan(&updatedUser); err != nil {
		slog.Error(
			"failed to update user",
			"updateData", updateData,
			"updated_struct", updatedUser,
			"error_message", err,
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return updatedUser, nil
}
