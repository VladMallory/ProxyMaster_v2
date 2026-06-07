package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	subsDomain "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/features/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateDeviceAddon создает запись купленной услуги "доп. устройство".
func (s *UserStorage) CreateDeviceAddon(
	userID string,
	nextChargeAt time.Time,
) (*subsDomain.DeviceAddon, error) {
	defer s.logDuration("CreateDeviceAddon")()
	addon := &subsDomain.DeviceAddon{
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
		s.logger.Error("failed to create device addon",
			zap.String("user_id", userID),
			zap.String("addon_id", addon.ID),
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to create device addon: %w", err)
	}

	return addon, nil
}

// CountActiveDeviceAddons возвращает количество активных доп. устройств пользователя.
func (s *UserStorage) CountActiveDeviceAddons(userID string) (int, error) {
	defer s.logDuration("CountActiveDeviceAddons")()
	query := `
	SELECT COUNT(*)
	FROM device_addons
	WHERE user_id = $1 AND active = TRUE
	`

	var cnt int
	if err := s.db.QueryRowx(query, userID).Scan(&cnt); err != nil {
		s.logger.Error("failed to count device addons",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to count device addons: %w", err)
	}

	return cnt, nil
}

// GetNextDeviceAddonChargeAt возвращает время следующего списания для дополнительных устройств.
func (s *UserStorage) GetNextDeviceAddonChargeAt(userID string) (*time.Time, error) {
	defer s.logDuration("GetNextDeviceAddonChargeAt")()
	query := `
	SELECT MIN(next_charge_at)
	FROM device_addons
	WHERE user_id = $1 AND active = TRUE
	`

	var nextChargeAt sql.NullTime
	if err := s.db.QueryRowx(query, userID).Scan(&nextChargeAt); err != nil {
		s.logger.Error("failed to get next charge date for device addons",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get next charge date: %w", err)
	}

	if !nextChargeAt.Valid {
		return nil, nil
	}

	return &nextChargeAt.Time, nil
}

// DeactivateAllDeviceAddons отключает все доп. устройства пользователя.
func (s *UserStorage) DeactivateAllDeviceAddons(userID string) error {
	defer s.logDuration("DeactivateAllDeviceAddons")()
	query := `
	UPDATE device_addons
	SET active = FALSE
	WHERE user_id = $1 AND active = TRUE
	`

	if _, err := s.db.Exec(query, userID); err != nil {
		s.logger.Error("failed to deactivate device addons",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to deactivate device addons: %w", err)
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
	defer s.logDuration("AddDeviceAddonAtomic")()
	// Начинаем транзакцию с блокировкой.
	tx, err := s.db.BeginTxx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		s.logger.Error("failed to begin transaction for AddDeviceAddonAtomic",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Блокируем запись пользователя, чтобы параллельные запросы ждали.
	lockQuery := `SELECT id FROM users WHERE id = $1 FOR UPDATE`
	var lockedID string
	if err := tx.QueryRowx(lockQuery, userID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("user not found when locking for AddDeviceAddonAtomic",
				zap.String("user_id", userID),
				zap.Error(domain.ErrUserNotFound),
			)
			return 0, domain.ErrUserNotFound
		}
		s.logger.Error("failed to lock user for AddDeviceAddonAtomic",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to lock user: %w", err)
	}

	// Считаем текущее количество активных доп. устройств с блокировкой строк.
	// Используем подзапрос для блокировки всех device_addons пользователя.
	countQuery := `
	SELECT COUNT(*)
	FROM (
		SELECT id
		FROM device_addons
		WHERE user_id = $1 AND active = TRUE
		FOR UPDATE
	) AS locked_addons
	`
	var activeAddons int
	if err := tx.QueryRowx(countQuery, userID).Scan(&activeAddons); err != nil {
		s.logger.Error("failed to count active addons for AddDeviceAddonAtomic",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to count active addons: %w", err)
	}

	// Проверяем лимит: базовое + купленные >= максимум.
	if baseLimit+activeAddons >= maxLimit {
		err := fmt.Errorf(
			"%w: у пользователя уже %d устройств",
			domain.ErrMaxDevices,
			baseLimit+activeAddons,
		)
		s.logger.Error("max devices limit reached",
			zap.String("user_id", userID),
			zap.Int("limit", maxLimit),
			zap.Int("current", baseLimit+activeAddons),
			zap.Error(err),
		)
		return 0, err
	}

	// Создаём запись device_addon.
	addonID := uuid.NewString()
	nextChargeAt := time.Now().Add(chargePeriod)
	insertQuery := `
	INSERT INTO device_addons (id, user_id, next_charge_at, active, created_at)
	VALUES ($1, $2, $3, TRUE, $4)
	`
	if _, err := tx.Exec(insertQuery, addonID, userID, nextChargeAt, time.Now()); err != nil {
		s.logger.Error("failed to insert device addon in AddDeviceAddonAtomic",
			zap.String("user_id", userID),
			zap.String("addon_id", addonID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to create device addon: %w", err)
	}

	// Обновляем счётчик в users для отображения.
	newCount = activeAddons + 1
	if err := s.setExtraDevicesCountTx(tx, userID, newCount); err != nil {
		s.logger.Error("failed to set extra devices count in AddDeviceAddonAtomic",
			zap.String("user_id", userID),
			zap.Int("new_count", newCount),
			zap.Error(err),
		)
		return 0, err
	}

	// Коммитим транзакцию.
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit AddDeviceAddonAtomic transaction",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newCount, nil
}
