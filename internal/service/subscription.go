package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
)

// SubscriptionService представляет собой сервис для управления подписками клиентов с помощью remnawave.
type SubscriptionService struct {
	remna  domain.RemnawaveClient
	dbRepo domain.UserRepository
	logger logger.Logger
}

// NewSubscriptionService конструктор сервиса.
func NewSubscriptionService(remna domain.RemnawaveClient, dbRepo domain.UserRepository, l logger.Logger) *SubscriptionService {
	l.Info("Создан экземпляр подписочного сервиса")
	return &SubscriptionService{
		remna:  remna,
		dbRepo: dbRepo,
		logger: l,
	}
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

// ActivateSubscription активирует подписку клиенту telegram на указанное количество месяцев.
// Если имеется подписка - продлить. Если подписки нет - создать.
func (s *SubscriptionService) ActivateSubscription(telegramID int64, months int) (string, error) {
	defer s.logDuration("ActivateSubscription")()

	// User id telegram клиента
	username := strconv.FormatInt(telegramID, 10)

	// Проверяем наличия пользователя в базе данных и создаем если его нету
	user, err := s.dbRepo.GetUserByID(username)

	// TODO: В идеале нужно различать ошибку "не найдено" и "ошибка БД"
	if err != nil {
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
	}

	totalDays := months * 30
	const pricePerMonth = 100
	// Расчитываем итоговую стоимость подписки
	totalCost := months * pricePerMonth

	// Проверяем баланс.
	if user.Balance < totalCost {
		s.logger.Info("у пользователя не достаточно средств для подписки", logger.Field{Key: "user_id", Value: username})
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
