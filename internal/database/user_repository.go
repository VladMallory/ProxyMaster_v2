// Package database отвечает за работу с Postgres (чтение/запись пользователей и услуг).
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"

	"github.com/google/uuid"
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
	SELECT id, balance, trial, extra_devices_count, created_at
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
	SELECT id, balance, trial, extra_devices_count, created_at
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

			// Возвращем ошибку о том что пользователя нет в DB
			return nil, domain.ErrUserNotFound
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
		slog.Error(
			"failed to update user",
			"updateData", updateData,
			"error_message", err,
		)

		return nil, fmt.Errorf("failed to scan struct: %w", err)
	}

	return &updatedUser, nil
}

// TryDebitBalance пытается списать amount с баланса пользователя атомарно.
// Если денег не хватает, ok=false и баланс не меняется.
func (s *UserStorage) TryDebitBalance(userID string, amount int) (newBalance int, ok bool, err error) {
	if amount <= 0 {
		return 0, false, fmt.Errorf("amount должен быть > 0")
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
		return 0, false, err
	}

	return bal, true, nil
}

// CreateDeviceAddon создает запись купленной услуги "доп. устройство".
func (s *UserStorage) CreateDeviceAddon(userID string, nextChargeAt time.Time) (*models.DeviceAddon, error) {
	addon := &models.DeviceAddon{
		ID:           uuid.NewString(),
		UserID:       userID,
		NextChargeAt: nextChargeAt,
		Active:       true,
		CreatedAt:    time.Now(),
	}

	query := `
	INSERT INTO device_addons (id, user_id, next_charge_at, active, created_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, user_id, next_charge_at, active, created_at
	`

	if err := s.db.QueryRowx(
		query,
		addon.ID,
		addon.UserID,
		addon.NextChargeAt,
		addon.Active,
		addon.CreatedAt,
	).StructScan(addon); err != nil {
		return nil, fmt.Errorf("failed to create device addon: %w", err)
	}

	return addon, nil
}

// CountActiveDeviceAddons возвращает количество активных доп. устройств пользователя.
func (s *UserStorage) CountActiveDeviceAddons(userID string) (int, error) {
	query := `
	SELECT COUNT(*)
	FROM device_addons
	WHERE user_id = $1 AND active = TRUE
	`

	var cnt int
	if err := s.db.QueryRowx(query, userID).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("failed to count device addons: %w", err)
	}

	return cnt, nil
}

// DeactivateAllDeviceAddons отключает все доп. устройства пользователя.
func (s *UserStorage) DeactivateAllDeviceAddons(userID string) error {
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE user_id = $1 AND active = TRUE
	`

	if _, err := s.db.Exec(query, userID); err != nil {
		return fmt.Errorf("failed to deactivate device addons: %w", err)
	}

	return nil
}

// ListDueActiveDeviceAddons возвращает активные услуги, у которых наступило списание.
func (s *UserStorage) ListDueActiveDeviceAddons(now time.Time, limit int) ([]models.DeviceAddon, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
	SELECT id, user_id, next_charge_at, active, created_at
	FROM device_addons
	WHERE active = TRUE AND next_charge_at <= $1
	ORDER BY next_charge_at ASC
	LIMIT $2
	`

	var addons []models.DeviceAddon
	if err := s.db.Select(&addons, query, now, limit); err != nil {
		return nil, fmt.Errorf("failed to list due device addons: %w", err)
	}

	return addons, nil
}

// UpdateDeviceAddonNextChargeAt переносит следующую дату списания.
func (s *UserStorage) UpdateDeviceAddonNextChargeAt(addonID string, nextChargeAt time.Time) error {
	query := `
	UPDATE device_addons
	SET next_charge_at = $1
	WHERE id = $2
	`

	if _, err := s.db.Exec(query, nextChargeAt, addonID); err != nil {
		return fmt.Errorf("failed to update device addon next_charge_at: %w", err)
	}

	return nil
}
