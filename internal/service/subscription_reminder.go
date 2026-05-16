package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

const (
	subscriptionReminderBefore = 72 * time.Hour
	subscriptionReminderAfter  = 48 * time.Hour
	subscriptionReminderPeriod = 24 * time.Hour
	activeUserStatus           = "ACTIVE"
)

type MessageSender interface {
	SendMessage(chatID int64, text string) error
}

type SubscriptionReminderService struct {
	remna  domain.RemnawaveClient
	sender MessageSender
	logger logger.Logger
	now    func() time.Time
}

func NewSubscriptionReminderService(
	remna domain.RemnawaveClient,
	sender MessageSender,
	logger logger.Logger,
) *SubscriptionReminderService {
	return &SubscriptionReminderService{
		remna:  remna,
		sender: sender,
		logger: logger,
		now:    time.Now,
	}
}

func (s *SubscriptionReminderService) RunDay(ctx context.Context) {
	// Запускаем проверку при запуске приложения
	s.Process(ctx)

	ticker := time.NewTicker(subscriptionReminderPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.Process(ctx)
		}
	}
}

func (s *SubscriptionReminderService) Process(ctx context.Context) {
	start := s.now()

	users, err := s.remna.GetAllUsers(ctx)
	if err != nil {
		s.logger.Error("не удалось получить пользователей RemnaWave",
			logger.Field{Key: "error", Value: err},
		)

		return
	}

	for _, user := range users {
		if ctx.Err() != nil {
			return
		}

		if user.ExpireAt == nil {
			continue
		}

		// Пишем пользователям у которых есть подписка
		if user.Status != activeUserStatus {
			continue
		}

		chatID, err := strconv.ParseInt(user.Username, 10, 64)
		if err != nil {
			s.logger.Error(
				"не удалось преобразовать username в telegram chat id",
				logger.Field{Key: "username", Value: user.Username},
				logger.Field{Key: "uuid", Value: user.UUID},
				logger.Field{Key: "error", Value: err},
			)

			continue
		}

		// Сколько осталось до конца подписки
		remaining := user.ExpireAt.Sub(start)

		// Проверка окна от 48 часов до 72 часов
		// Сообщение уйдет примерно за 3 дня, и не будет отправляться до окончания подписки
		if remaining > subscriptionReminderBefore || remaining <= 0 {
			continue
		}

		message := s.buildReminderMessage(user.ExpireAt)

		if err := s.sender.SendMessage(chatID, message); err != nil {
			s.logger.Error(
				"не удалось отправить напоминание о подписке",
				logger.Field{Key: "username", Value: user.Username},
				logger.Field{Key: "uuid", Value: user.UUID},
				logger.Field{Key: "error", Value: err},
			)

			continue
		}

		s.logger.Info(
			"Пользователю отправлено сообщение о продлении подписки",
			logger.Field{Key: "username", Value: chatID},
		)
	}
}

func (s *SubscriptionReminderService) buildReminderMessage(expireAt *time.Time) string {
	expireDate := expireAt.Format("02.01.2006")

	return fmt.Sprintf(
		"Ваша подписка заканчивается %s\n\n",
		expireDate,
	)
}
