package database

import (
	"database/sql"
	"log/slog"
	"time"

	"ProxyMaster_v2/internal/models"

	"github.com/jmoiron/sqlx"
)

type UserStorage struct {
	db *sqlx.DB
}

func NewUserStorage(db *sqlx.DB) *UserStorage {
	return &UserStorage{
		db: db,
	}
}

func (s *UserStorage) CreateUser(user_data models.CreateUserTGDTO) (*models.UserTG, error) {
	var user models.UserTG

	query := `
	INSERT INTO users (id, balance, trial, created_at)
	VALUES ($1, $2, $3, $4)
	RETURNING id, balance, trial, created_at
	`

	now := time.Now()
	err := s.db.QueryRowx(
		query,
		user_data.ID,
		user_data.Balance,
		user_data.Trial,
		now,
	).StructScan(&user)
	if err != nil {
		slog.Error(
			"failed to create user",
			"error_message", err,
		)
		return nil, err
	}
	return &user, nil
}

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
		return nil, err
	}
	return users, nil
}

func (s *UserStorage) GetUserByID(id string) (*models.UserTG, error) {
	var user models.UserTG
	query := `
	SELECT id, balance, trial, created_at
	FROM users
	WHERE id = $1
	`

	if err := s.db.Get(&user, query, id); err != nil {
		if err == sql.ErrNoRows {
			slog.Error(
				"user not found",
				"id", id,
				"error_message", err,
			)
			return nil, err
		}
		slog.Error(
			"failed to get user",
			"id", id,
			"error_message", err,
		)
	}
	return &user, nil
}

//обновляет юзера, можно че кайф обновить(пока только баланс или триал), логику того, что обновляем будет писать в хэндлере бота

func (s *UserStorage) UpdateUser(id string, update_data models.UpdateUserTGDTO) (*models.UserTG, error) {
	var updatedUser *models.UserTG

	user, err := s.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	if update_data.Balance != nil {
		user.Balance = *update_data.Balance
	}

	if update_data.Trial != nil {
		user.Trial = *update_data.Trial
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
			"update_data", update_data,
			"updated_struct", updatedUser,
			"error_message", err,
		)
		return nil, err
	}
	return updatedUser, nil
}
