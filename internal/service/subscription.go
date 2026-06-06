// Package service содержит бизнес-логику проекта (подписки, услуги, списания).
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/infrastructure/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"go.uber.org/zap"
)

const (
	maxDevicesLimit     = 5
	extraDevicePriceRUB = 50
)

// SubscriptionService представляет собой сервис для управления подписками клиентов с помощью remnawave.
type SubscriptionService struct {
	remna           domain.RemnawaveClient
	dbRepo          *database.UserStorage
	logger          *zap.Logger
	baseDeviceLimit int // Базовый лимит устройств (из DEVICE_LIMIT)
}

// NewSubscriptionService конструктор сервиса.
func NewSubscriptionService(
	remna domain.RemnawaveClient,
	dbRepo *database.UserStorage,
	l *zap.Logger,
	baseDeviceLimit int,
) *SubscriptionService {
	svc := &SubscriptionService{
		remna:           remna,
		dbRepo:          dbRepo,
		logger:          l,
		baseDeviceLimit: baseDeviceLimit,
	}

	// Запускаем фоновую проверку биллинга доп. устройств.
	go svc.runExtraDevicesBillingLoop()
	// Автопродление подписок отключено, так как баланс удален.
	// go svc.runSubscriptionBillingLoop()

	return svc
}

// GetURLSubscription получает url подписки пользователя через userID.
func (s *SubscriptionService) GetURLSubscription(userID string) (string, error) {
	// Получаем UUID пользователя по userID
	uuid, err := s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		return "", fmt.Errorf("не удалось получить UUID пользователя: %w", err)
	}

	// Получаем информацию о пользователе по UUID
	userInfo, err := s.remna.GetUserInfo(uuid)
	if err != nil {
		return "", fmt.Errorf("не удалось получить информацию о пользователе: %w", err)
	}

	return userInfo.Response.SubscriptionURL, nil
}

func (s *SubscriptionService) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info("вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

// logError логирует ошибку и возвращает её обернутую.
func (s *SubscriptionService) logError(msg string, err error, fields ...zap.Field) error {
	// Добавляем ошибку к полям
	allFields := append([]zap.Field{zap.Error(err)}, fields...)
	s.logger.Error(msg, allFields...)

	return fmt.Errorf("%s: %w", msg, err)
}

// AddPaidDevice покупает пользователю 1 доп. устройство за 50₽/мес.
// Использует атомарную операцию для защиты от race condition при параллельных запросах.
// nolint: funlen
func (s *SubscriptionService) AddPaidDevice(userID string) error {
	defer s.logDuration("AddPaidDevice")()

	// Требуем чтобы пользователь существовал в DB
	_, err := s.dbRepo.GetUserByID(userID)
	if err != nil {
		return s.logError(
			"пользователь не найден",
			err,
			zap.String("user_id", userID),
		)
	}

	// Проверяем наличие пользователя в RemnaWave, создаем если нет.
	_, err = s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		if errors.Is(err, remnawave.ErrNotFound) {
			s.logger.Info(
				"пользователь не найден в RemnaWave при покупке устройства, создаем нового",
				zap.String("user_id", userID),
			)

			dto := remnawave.CreateUserDTO{
				Username: userID,
				Days:     30,
			}

			err = s.remna.CreateUser(dto)
			if err != nil {
				return s.logError(
					"ошибка создания пользователя в RemnaWave при покупке устройства",
					err,
					zap.String("user_id", userID),
				)
			}
		} else {
			return s.logError(
				"ошибка поиска пользователя в RemnaWave при покупке устройства",
				err,
				zap.String("user_id", userID),
			)
		}
	}

	// Атомарно: проверяем лимит, списываем деньги, создаём addon, обновляем счётчик.
	// Используем базовый лимит из конфигурации вместо текущего лимита из RemnaWave
	newExtraDevicesCount, err := s.dbRepo.AddDeviceAddonAtomic(
		userID,
		s.baseDeviceLimit,
		maxDevicesLimit,
		extraDevicePriceRUB,
		30*24*time.Hour,
	)
	if err != nil {
		if errors.Is(err, domain.ErrMaxDevices) || errors.Is(err, domain.ErrInsufficientFunds) {
			s.logger.Error("ошибка добавления доп. устройства",
				zap.String("user_id", userID),
				zap.Error(err),
			)

			return fmt.Errorf("ошибка добавления доп устройст: %w", err)
		}

		return s.logError(
			"ошибка добавления доп. устройства",
			err,
			zap.String("user_id", userID),
		)
	}

	// Проставляем лимит устройств в RemnaWave: базовый + купленные доп. устройства
	devices := uint8(s.baseDeviceLimit + newExtraDevicesCount)
	if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
		return s.logError(
			"ошибка установки устройств в remnawave",
			err,
			zap.String("user_id", userID),
		)
	}

	return nil
}

// ResetPaidDevices сбрасывает услугу доп. устройств в 0 и ставит 1 устройство в RemnaWave.
func (s *SubscriptionService) ResetPaidDevices(userID string) error {
	defer s.logDuration("ResetPaidDevices")()

	// Отключаем все купленные услуги.
	if err := s.dbRepo.DeactivateAllDeviceAddons(userID); err != nil {
		return s.logError(
			"ошибка установки устройств в remnawave",
			err,
			zap.String("user_id", userID),
		)
	}

	// Обнуляем счетчик для отображения.
	zero := 0

	_, err := s.dbRepo.UpdateUser(userID, models.UpdateUserTGDTO{ExtraDevicesCount: &zero})
	if err != nil {
		return s.logError(
			"ошибка сброса услуг доп. устройств",
			err,
			zap.String("user_id", userID),
		)
	}

	// Ставим базовый лимит RemnaWave
	devices := uint8(s.baseDeviceLimit)
	if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
		return s.logError(
			"ошибка установки устройств в remnawave",
			err,
			zap.String("user_id", userID),
		)
	}

	return nil
}

func (s *SubscriptionService) PrepayPaidDevices(userID string) (int, error) {
	defer s.logDuration("PrepayPaidDevices")()

	count, err := s.dbRepo.PrepayDeviceAddonsAtomic(
		userID,
		extraDevicePriceRUB,
		30*24*time.Hour,
	)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) ||
			errors.Is(err, domain.ErrNoActiveDeviceAddons) {
			s.logger.Error("ошибка предоплаты доп. устройств",
				zap.String("user_id", userID),
				zap.Error(err),
			)

			return 0, fmt.Errorf("ошибка предоплаты доп устройств: %w", err)
		}

		return 0, s.logError(
			"ошибка предоплаты доп. устройств",
			err,
			zap.String("user_id", userID),
		)
	}

	return count, nil
}

// runExtraDevicesBillingLoop раз в сутки проверяет и деактивирует просроченные доп. устройства.
func (s *SubscriptionService) runExtraDevicesBillingLoop() {
	s.processExtraDevicesBilling(time.Now())

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.processExtraDevicesBilling(time.Now())
	}
}

func (s *SubscriptionService) processExtraDevicesBilling(now time.Time) {
	activeCounts, err := s.dbRepo.ProcessDueDeviceAddonsBilling(
		now,
		200,
		extraDevicePriceRUB,
		30*24*time.Hour,
	)
	if err != nil {
		s.logger.Error("ошибка биллинга доп. устройств", zap.Error(err))

		return
	}

	for userID, activeAddons := range activeCounts {
		// Устанавливаем лимит в RemnaWave: базовый + активные доп. устройства.
		devices := uint8(s.baseDeviceLimit + activeAddons)
		if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
			s.logger.Error(
				"ошибка установки лимита устройств в remnawave",
				zap.String("user_id", userID),
				zap.Uint8("devices", devices),
				zap.Error(err),
			)
		}
	}
}

func (s *SubscriptionService) runSubscriptionBillingLoop() {
	s.processSubscriptionBilling()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.processSubscriptionBilling()
	}
}

func (s *SubscriptionService) processSubscriptionBilling() {
	userIDs, err := s.dbRepo.GetActiveUserIDs()
	if err != nil {
		s.logger.Error(
			"ошибка получения пользователей для автопродления",
			zap.Error(err),
		)

		return
	}

	for _, userID := range userIDs {
		if renewErr := s.tryAutoRenewSubscription(userID); renewErr != nil {
			s.logger.Error(
				"ошибка автопродления подписки",
				zap.String("user_id", userID),
				zap.Error(renewErr),
			)
		}
	}
}

func (s *SubscriptionService) tryAutoRenewSubscription(userID string) error {
	_, err := s.ActivateSubscription(userID, 1)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) {
			s.logger.Info(
				"недостаточно средств для автопродления",
				zap.String("user_id", userID),
			)

			return nil
		}

		return err
	}

	return nil
}

// ActivateSubscription активирует подписку на указанное количество месяцев.
// Если имеется подписка - продлить. Если подписки нет - создать.
// nolint:funlen
func (s *SubscriptionService) ActivateSubscription(
	userID string,
	months int,
) (string, error) {
	defer s.logDuration("ActivateSubscription")()

	_, err := s.dbRepo.GetUserByID(userID)
	if err != nil {
		return "", s.logError(
			"пользователь не найден в DB",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("пользователь найден", zap.String("user_id", userID))

	// Вычисляем на сколько дней клиенту нужна подписка
	totalDays := months * 30

	// Вычисляем стоимость подписки за указанное количество месяцев
	// Проверяем есть ли пользователь в панели
	userUUID, err := s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		// Если пользователя нет, создаем его в панели
		if errors.Is(err, remnawave.ErrNotFound) {
			s.logger.Info(
				"пользователь не найден, создаем нового",
				zap.String("user_id", userID),
			)

			dto := remnawave.CreateUserDTO{
				Username: userID,
				Days:     totalDays,
			}

			err = s.remna.CreateUser(dto)
			if err != nil {
			return "", s.logError(
				"ошибка создания пользователя",
				err,
				zap.String("user_id", userID),
			)
			}

			return fmt.Sprintf("пользователь %s создан на %d дней", userID, totalDays), nil
		}

		return "", s.logError(
			"ошибка поиска пользователя",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("пользователь найден", zap.String("user_id", userID))

	// Получаем информацию о пользователе, чтобы проверить expireAt
	userInfo, err := s.remna.GetUserInfo(userUUID)
	if err != nil {
		return "", s.logError(
			"ошибка получения информации о пользователе",
			err,
			zap.String("user_id", userID),
		)
	}

	// Если подписка истекла добавляем дни просрочки к totalDays.
	// Это нужно, чтобы RemnaWave добавил дни к старому expireAt,
	// и в итоге новый expireAt оказался = now + оплаченные дни.
	if userInfo.Response.ExpireAt.Before(time.Now()) {
		daysSinceExpiry := int(time.Since(userInfo.Response.ExpireAt).Hours()/24) + 1
		totalDays += daysSinceExpiry
	}

	err = s.remna.ExtendClientSubscription(userUUID, userID, totalDays)
	if err != nil {
		return "", s.logError(
			"ошибка продления подписки",
			err,
			zap.String("user_id", userID),
		)
	}

	s.logger.Info("подписка продлена",
		zap.String("user_id", userID),
		zap.Int("days", totalDays),
	)

	return "подписка для пользователя " + userID + " продлена на " + strconv.Itoa(
		totalDays,
	) + " дней", nil
}

// AddDevice добавляет 1 устройство пользователю.
func (s *SubscriptionService) AddDevice(userID string) error {
	defer s.logDuration("AddDevice")()

	uuid, err := s.remna.GetUUIDByUsername(context.Background(), userID)
	if err != nil {
		s.logger.Error(
			"failed to get user UUID",
			zap.Error(err),
		)

		return fmt.Errorf("failed to get user: %w", err)
	}

	user, err := s.remna.GetUserInfo(uuid)
	if err != nil {
		s.logger.Error(
			"failed to user by UUID",
			zap.Error(err),
		)

		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Проверяем сколько уже устройств
	if user.Response.HWIDDeviceLimit >= 5 {
		s.logger.Info(
			"у клиента уже максимальное количество устройств",
			zap.String("user", userID),
		)

		return fmt.Errorf(
			"%w. У пользователя уже %d устройств",
			domain.ErrMaxDevices,
			user.Response.HWIDDeviceLimit,
		)
	}

	devices := uint8(user.Response.HWIDDeviceLimit) + 1
	if err := s.remna.SetDevices(context.Background(), userID, &devices); err != nil {
		s.logger.Error(
			"failed to set device limit",
			zap.Error(err),
		)

		return fmt.Errorf("failed to set devices: %w", err)
	}

	s.logger.Info(
		"успешное добавление устройства в подписку клиента",
		zap.String("user", userID),
		zap.Uint8("devices", devices),
	)

	return nil
}
