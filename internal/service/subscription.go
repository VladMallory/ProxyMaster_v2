package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/pkg/logger"
)

// SubscriptionService представляет собой сервис для управления подписками клиентов с помощью remnawave.
type SubscriptionService struct {
	remna  domain.RemnawaveClient
	logger logger.Logger
}

// NewSubscriptionService конструктор сервиса.
func NewSubscriptionService(remna domain.RemnawaveClient, l logger.Logger) *SubscriptionService {
	l.Info("Создан экземпляр подписочного сервиса")
	return &SubscriptionService{
		remna:  remna,
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

	totalDays := months * 30
	username := strconv.FormatInt(telegramID, 10)

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
