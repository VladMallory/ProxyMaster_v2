// Package service содержит бизнес-логику проекта (подписки, услуги, списания).
package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
)

const (
	baseDevicesLimit    = 1
	maxDevicesLimit     = 5
	extraDevicePriceRUB = 50
)

// SubscriptionService представляет собой сервис для управления подписками клиентов с помощью remnawave.
type SubscriptionService struct {
	remna  domain.RemnawaveClient
	dbRepo *database.UserStorage
	logger logger.Logger
}

// NewSubscriptionService конструктор сервиса.
func NewSubscriptionService(remna domain.RemnawaveClient, dbRepo *database.UserStorage, l logger.Logger) *SubscriptionService {
	l.Info("Создан экземпляр подписочного сервиса")

	svc := &SubscriptionService{
		remna:  remna,
		dbRepo: dbRepo,
		logger: l,
	}

	go svc.runExtraDevicesBillingLoop()

	return svc
}

func (s *SubscriptionService) logDuration(method string) func() {
	start := time.Now()

	return func() {
		s.logger.Info("вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

// logError логирует ошибку и возвращает её обернутую.
func (s *SubscriptionService) logError(msg string, err error, fields ...logger.Field) error {
	// Добавляем ошибку к полям
	allFields := append([]logger.Field{{Key: "error", Value: err}}, fields...)
	s.logger.Error(msg, allFields...)
	return fmt.Errorf("%s: %w", msg, err)
}

// AddPaidDevice покупает пользователю 1 доп. устройство за 50₽/мес.
// Использует атомарную операцию для защиты от race condition при параллельных запросах.
func (s *SubscriptionService) AddPaidDevice(username string) error {
	defer s.logDuration("AddPaidDevice")()

	// Атомарно: проверяем лимит, списываем деньги, создаём addon, обновляем счётчик.
	newCount, err := s.dbRepo.AddDeviceAddonAtomic(
		username,
		baseDevicesLimit,
		maxDevicesLimit,
		extraDevicePriceRUB,
		30*24*time.Hour,
	)
	if err != nil {
		if errors.Is(err, domain.ErrMaxDevices) || errors.Is(err, domain.ErrInsufficientFunds) {
			s.logger.Error("ошибка добавления доп. устройства",
				logger.Field{Key: "user_id", Value: username},
				logger.Field{Key: "error", Value: err},
			)
			return err
		}
		return s.logError("ошибка добавления доп. устройства", err, logger.Field{Key: "user_id", Value: username})
	}

	// Проставляем лимит устройств в RemnaWave: базовое + купленные.
	devices := uint8(baseDevicesLimit + newCount)
	if err := s.remna.SetDevices(username, &devices); err != nil {
		return s.logError("ошибка установки устройств в remnawave", err, logger.Field{Key: "user_id", Value: username})
	}

	return nil
}

// ResetPaidDevices сбрасывает услугу доп. устройств в 0 и ставит 1 устройство в RemnaWave.
func (s *SubscriptionService) ResetPaidDevices(username string) error {
	defer s.logDuration("ResetPaidDevices")()

	// Отключаем все купленные услуги.
	if err := s.dbRepo.DeactivateAllDeviceAddons(username); err != nil {
		return s.logError("ошибка сброса услуг доп. устройств", err, logger.Field{Key: "user_id", Value: username})
	}

	// Обнуляем счетчик для отображения.
	zero := 0
	_, err := s.dbRepo.UpdateUser(username, models.UpdateUserTGDTO{
		ExtraDevicesCount: &zero,
	})
	if err != nil {
		return s.logError("ошибка обновления счетчика доп. устройств", err, logger.Field{Key: "user_id", Value: username})
	}

	// Ставим всегда 1 устройство, как ты просил.
	devices := uint8(baseDevicesLimit)
	if err := s.remna.SetDevices(username, &devices); err != nil {
		return s.logError("ошибка установки устройств в remnawave", err, logger.Field{Key: "user_id", Value: username})
	}

	return nil
}

// runExtraDevicesBillingLoop раз в час проверяет и списывает доп. устройства.
func (s *SubscriptionService) runExtraDevicesBillingLoop() {
	s.processExtraDevicesBilling(time.Now())

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.processExtraDevicesBilling(time.Now())
	}
}

func (s *SubscriptionService) processExtraDevicesBilling(now time.Time) {
	usersToReset, err := s.dbRepo.ProcessDueDeviceAddonsBilling(now, 200, extraDevicePriceRUB, 30*24*time.Hour)
	if err != nil {
		s.logger.Error("ошибка биллинга доп. устройств", logger.Field{Key: "err_msg", Value: err})
		return
	}

	for _, userID := range usersToReset {
		devices := uint8(baseDevicesLimit)
		if err := s.remna.SetDevices(userID, &devices); err != nil {
			s.logger.Error(
				"ошибка установки базового лимита устройств в remnawave",
				logger.Field{Key: "user_id", Value: userID},
				logger.Field{Key: "err_msg", Value: err},
			)
		}
	}
}

// ActivateSubscription активирует подписку клиенту telegram на указанное количество месяцев.
// Если имеется подписка - продлить. Если подписки нет - создать.
func (s *SubscriptionService) ActivateSubscription(telegramID int64, months int) (string, error) {
	defer s.logDuration("ActivateSubscription")()

	// User id telegram клиента
	username := strconv.FormatInt(telegramID, 10)

	// Проверяем наличия пользователя в базе данных и создаем если его нет
	user, err := s.dbRepo.GetUserByID(username)

	if err != nil {
		// Проверяем, является ли ошибка "пользователь не найден"
		if errors.Is(err, domain.ErrUserNotFound) {
			s.logger.Info("пользователь не найден в DB, создаем нового", logger.Field{Key: "user_id", Value: username})

			// Делаем запрос DB на создание пользователя
			// Записываем в newUser данные которые получили от DB
			newUser, createDBErr := s.dbRepo.CreateUser(models.CreateUserTGDTO{
				ID:      username,
				Balance: 0,
				Trial:   false,
			})
			if createDBErr != nil {
				return "", s.logError("ошибка создания пользователя в DB", createDBErr, logger.Field{Key: "user_id", Value: username})
			}

			user = newUser
		} else {

			// Если пользователь не найден, скорее всего это ошибка DB
			return "", s.logError("ошибка поиска пользователя в DB", err, logger.Field{Key: "user_id", Value: username})
		}

	}

	s.logger.Info("пользователь найден", logger.Field{Key: "user_id", Value: username})

	// Вычисляем на сколько дней клиенту нужна подписка
	totalDays := months * 30
	const pricePerMonth = 100

	// Вычисляем стоимость подписки за указанное количество месяцев
	// Если пришла 2 месяца, 100 * на 2 = 200 итоговая цена
	totalCost := months * pricePerMonth

	// Проверяем достаточно ли на балансе средств на подписку
	if user.Balance < totalCost {
		s.logger.Info("у пользователя не достаточно средств для подписки",
			logger.Field{Key: "user_id", Value: username},
			logger.Field{Key: "balance", Value: user.Balance},
			logger.Field{Key: "required", Value: totalCost},
		)
		return "", fmt.Errorf("%w. Баланс: %d ₽, Требуется: %d ₽", domain.ErrInsufficientFunds, user.Balance, totalCost)
	}

	// Списываем средства
	newBalance := user.Balance - totalCost
	_, err = s.dbRepo.UpdateUser(username, models.UpdateUserTGDTO{
		Balance: &newBalance,
	})
	if err != nil {
		return "", s.logError("ошибка обновления баланса пользователя в DB", err, logger.Field{Key: "user_id", Value: username})
	}

	// Проверяем есть ли пользователь в панели
	userUUID, err := s.remna.GetUUIDByUsername(username)
	if err != nil {
		// Если пользователя нет, создаем его в панели
		if errors.Is(err, remnawave.ErrNotFound) {
			s.logger.Info("пользователь не найден, создаем нового", logger.Field{Key: "username", Value: username})
			err = s.remna.CreateUser(username, totalDays)
			if err != nil {
				return "", s.logError("ошибка создания пользователя", err, logger.Field{Key: "username", Value: username})
			}

			return fmt.Sprintf("пользователь %s создан на %d дней", username, totalDays), nil
		}

		return "", s.logError("ошибка поиска пользователя", err, logger.Field{Key: "username", Value: username})
	}

	s.logger.Info("пользователь найден", logger.Field{Key: "username", Value: username})

	err = s.remna.ExtendClientSubscription(userUUID, username, totalDays)
	if err != nil {
		return "", s.logError("ошибка продления подписки", err, logger.Field{Key: "username", Value: username})
	}

	s.logger.Info("подписка продлена",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "days", Value: totalDays},
	)
	return "подписка для пользователя " + username + " продлена на " + strconv.Itoa(totalDays) + " дней", nil
}

// AddDevice добавляет 1 устройство пользователю
func (s *SubscriptionService) AddDevice(username string) error {
	defer s.logDuration("AddDevice")()

	uuid, err := s.remna.GetUUIDByUsername(username)
	if err != nil {
		s.logger.Error(
			"failed to get user UUID",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	user, err := s.remna.GetUserInfo(uuid)
	if err != nil {
		s.logger.Error(
			"failed to user by UUID",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	// Проверяем сколько уже устройств
	if user.Response.HWIDDeviceLimit >= 5 {
		s.logger.Info(
			"у клиента уже максимальное количество устройств",
			logger.Field{Key: "user", Value: username},
		)

		return fmt.Errorf("%w. У пользователя уже %d устройств", domain.ErrMaxDevices, user.Response.HWIDDeviceLimit)
	}

	devices := uint8(user.Response.HWIDDeviceLimit) + 1
	if err := s.remna.SetDevices(username, &devices); err != nil {
		s.logger.Error(
			"failed to set device limit",
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	s.logger.Info(
		"успешное добавление устройства в подписку клиента",
		logger.Field{Key: "user", Value: username},
		logger.Field{Key: "devices", Value: devices},
	)

	return nil
}
