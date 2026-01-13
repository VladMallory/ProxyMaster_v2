package database

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
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
func (s *UserStorage) UpdateUser(
	id string,
	updateData models.UpdateUserTGDTO,
) (*models.UserTG, error) {
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
func (s *UserStorage) TryDebitBalance(
	userID string,
	amount int,
) (newBalance int, ok bool, err error) {
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
func (s *UserStorage) CreateDeviceAddon(
	userID string,
	nextChargeAt time.Time,
) (*models.DeviceAddon, error) {
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

type dueDeviceAddonRow struct {
	ID     string `db:"id"`
	UserID string `db:"user_id"`
}

func (s *UserStorage) ProcessDueDeviceAddonsBilling(
	now time.Time,
	limit int,
	priceRUB int,
	chargePeriod time.Duration,
) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	if priceRUB <= 0 {
		return nil, fmt.Errorf("priceRUB должен быть > 0")
	}
	if chargePeriod <= 0 {
		return nil, fmt.Errorf("chargePeriod должен быть > 0")
	}

	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selectUsersQuery := `
SELECT DISTINCT ON (user_id) user_id
FROM device_addons
WHERE active = TRUE AND next_charge_at <= $1
ORDER BY user_id, next_charge_at ASC
LIMIT $2
`

	var userIDs []string
	if err := tx.Select(&userIDs, selectUsersQuery, now, limit); err != nil {
		return nil, fmt.Errorf("failed to select due device addon users: %w", err)
	}
	if len(userIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit empty billing transaction: %w", err)
		}
		return nil, nil
	}

	selectDueForUsersQuery := `
	SELECT id, user_id
	FROM device_addons
	WHERE active = TRUE AND next_charge_at <= $1 AND user_id = ANY($2)
	ORDER BY user_id, next_charge_at ASC
	FOR UPDATE
	`

	var due []dueDeviceAddonRow
	if err := tx.Select(&due, selectDueForUsersQuery, now, pq.Array(userIDs)); err != nil {
		return nil, fmt.Errorf("failed to select due device addons for users: %w", err)
	}

	byUser := make(map[string][]string, len(userIDs))
	for _, row := range due {
		byUser[row.UserID] = append(byUser[row.UserID], row.ID)
	}

	nextChargeAt := now.Add(chargePeriod)

	usersToReset := make([]string, 0)
	for userID, addonIDs := range byUser {
		if len(addonIDs) == 0 {
			continue
		}
		amount := len(addonIDs) * priceRUB

		_, ok, err := tryDebitBalanceTx(tx, userID, amount)
		if err != nil {
			return nil, err
		}

		if !ok {
			if err := deactivateAllDeviceAddonsTx(tx, userID); err != nil {
				return nil, err
			}
			if err := setExtraDevicesCountTx(tx, userID, 0); err != nil {
				return nil, err
			}
			usersToReset = append(usersToReset, userID)
			continue
		}

		if err := updateDeviceAddonsNextChargeAtTx(tx, addonIDs, nextChargeAt); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return usersToReset, nil
}

func tryDebitBalanceTx(
	tx *sqlx.Tx,
	userID string,
	amount int,
) (newBalance int, ok bool, err error) {
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
	if err := tx.QueryRowx(query, amount, userID).Scan(&bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to debit balance: %w", err)
	}

	return bal, true, nil
}

func deactivateAllDeviceAddonsTx(tx *sqlx.Tx, userID string) error {
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE user_id = $1 AND active = TRUE
	`
	if _, err := tx.Exec(query, userID); err != nil {
		return fmt.Errorf("failed to deactivate device addons: %w", err)
	}
	return nil
}

func setExtraDevicesCountTx(tx *sqlx.Tx, userID string, cnt int) error {
	query := `
	UPDATE users
	SET extra_devices_count = $1
	WHERE id = $2
	`
	if _, err := tx.Exec(query, cnt, userID); err != nil {
		return fmt.Errorf("failed to update extra_devices_count: %w", err)
	}
	return nil
}

func updateDeviceAddonsNextChargeAtTx(
	tx *sqlx.Tx,
	addonIDs []string,
	nextChargeAt time.Time,
) error {
	query := `
	UPDATE device_addons
	SET next_charge_at = $1
	WHERE id = ANY($2)
	`
	if _, err := tx.Exec(query, nextChargeAt, pq.Array(addonIDs)); err != nil {
		return fmt.Errorf("failed to update next_charge_at: %w", err)
	}
	return nil
}

// AddDeviceAddonAtomic атомарно добавляет доп. устройство пользователю.
// Выполняет в одной транзакции: проверку лимита, списание денег, создание addon, обновление счётчика.
// Возвращает новое количество активных устройств (без базового).
func (s *UserStorage) AddDeviceAddonAtomic(
	userID string,
	baseLimit, maxLimit, priceRUB int,
	chargePeriod time.Duration,
) (newCount int, err error) {
	// Начинаем транзакцию с блокировкой.
	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Блокируем запись пользователя, чтобы параллельные запросы ждали.
	lockQuery := `SELECT id FROM users WHERE id = $1 FOR UPDATE`
	var lockedID string
	if err := tx.QueryRowx(lockQuery, userID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.ErrUserNotFound
		}
		return 0, fmt.Errorf("failed to lock user: %w", err)
	}

	// Считаем текущее количество активных доп. устройств.
	countQuery := `SELECT COUNT(*) FROM device_addons WHERE user_id = $1 AND active = TRUE`
	var activeAddons int
	if err := tx.QueryRowx(countQuery, userID).Scan(&activeAddons); err != nil {
		return 0, fmt.Errorf("failed to count active addons: %w", err)
	}

	// Проверяем лимит: базовое + купленные >= максимум.
	if baseLimit+activeAddons >= maxLimit {
		return 0, fmt.Errorf(
			"%w: у пользователя уже %d устройств",
			domain.ErrMaxDevices,
			baseLimit+activeAddons,
		)
	}

	// Пытаемся списать деньги.
	_, ok, err := tryDebitBalanceTx(tx, userID, priceRUB)
	if err != nil {
		return 0, fmt.Errorf("failed to debit balance: %w", err)
	}
	if !ok {
		return 0, domain.ErrInsufficientFunds
	}

	// Создаём запись device_addon.
	addonID := uuid.NewString()
	nextChargeAt := time.Now().Add(chargePeriod)
	insertQuery := `
	INSERT INTO device_addons (id, user_id, next_charge_at, active, created_at)
	VALUES ($1, $2, $3, TRUE, $4)
	`
	if _, err := tx.Exec(insertQuery, addonID, userID, nextChargeAt, time.Now()); err != nil {
		return 0, fmt.Errorf("failed to create device addon: %w", err)
	}

	// Обновляем счётчик в users для отображения.
	newCount = activeAddons + 1
	if err := setExtraDevicesCountTx(tx, userID, newCount); err != nil {
		return 0, err
	}

	// Коммитим транзакцию.
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newCount, nil
}

//
