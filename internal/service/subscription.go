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
func (s *SubscriptionService) AddPaidDevice(username string) error {
	defer s.logDuration("AddPaidDevice")()

	// Считаем активные доп. устройства, чтобы не выйти за общий лимит.
	activeAddons, err := s.dbRepo.CountActiveDeviceAddons(username)
	if err != nil {
		return s.logError("ошибка подсчета доп. устройств", err, logger.Field{Key: "user_id", Value: username})
	}

	// Проверяем лимит: базовое 1 + купленные.
	if baseDevicesLimit+activeAddons >= maxDevicesLimit {
		return fmt.Errorf("%w. У пользователя уже %d устройств", domain.ErrMaxDevices, baseDevicesLimit+activeAddons)
	}

	// Пытаемся списать деньги атомарно (если не хватает — просто возвращаем ошибку).
	_, ok, err := s.dbRepo.TryDebitBalance(username, extraDevicePriceRUB)
	if err != nil {
		return s.logError("ошибка списания средств", err, logger.Field{Key: "user_id", Value: username})
	}
	if !ok {
		return domain.ErrInsufficientFunds
	}

	// Создаем отдельную запись услуги с собственным расписанием.
	_, err = s.dbRepo.CreateDeviceAddon(username, time.Now().Add(30*24*time.Hour))
	if err != nil {
		return s.logError("ошибка создания услуги доп. устройства", err, logger.Field{Key: "user_id", Value: username})
	}

	// Обновляем счетчик в users для отображения.
	newCount := activeAddons + 1
	_, err = s.dbRepo.UpdateUser(username, models.UpdateUserTGDTO{
		ExtraDevicesCount: &newCount,
	})
	if err != nil {
		return s.logError("ошибка обновления счетчика доп. устройств", err, logger.Field{Key: "user_id", Value: username})
	}

	// Проставляем лимит устройств в RemnaWave: 1 + купленные.
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
	devices := uint8(1)
	if err := s.remna.SetDevices(username, &devices); err != nil {
		return s.logError("ошибка установки устройств в remnawave", err, logger.Field{Key: "user_id", Value: username})
	}

	return nil
}

// runExtraDevicesBillingLoop раз в час проверяет и списывает доп. устройства.
func (s *SubscriptionService) runExtraDevicesBillingLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Берем пачку услуг, у которых наступило время списания.
		addons, err := s.dbRepo.ListDueActiveDeviceAddons(now, 200)
		if err != nil {
			s.logger.Error("ошибка получения услуг для биллинга", logger.Field{Key: "err_msg", Value: err})
			continue
		}

		for _, addon := range addons {
			// Пытаемся списать 50₽ за конкретную услугу.
			_, ok, err := s.dbRepo.TryDebitBalance(addon.UserID, extraDevicePriceRUB)
			if err != nil {
				s.logger.Error("ошибка списания за доп. устройство", logger.Field{Key: "user_id", Value: addon.UserID}, logger.Field{Key: "err_msg", Value: err})
				continue
			}

			// Если денег не хватило — сбрасываем услугу в 0.
			if !ok {
				_ = s.ResetPaidDevices(addon.UserID)
				continue
			}

			// Переносим дату следующего списания на месяц вперед.
			next := now.Add(30 * 24 * time.Hour)
			if err := s.dbRepo.UpdateDeviceAddonNextChargeAt(addon.ID, next); err != nil {
				s.logger.Error("ошибка переноса next_charge_at", logger.Field{Key: "user_id", Value: addon.UserID}, logger.Field{Key: "err_msg", Value: err})
				continue
			}
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
