package reminders

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"go.uber.org/zap"
)

const (
	reminderWindow = 72 * time.Hour
	checkPeriod    = 24 * time.Hour
	activeStatus   = "ACTIVE"
)

// ReminderSender — абстракция для отправки.
// Реализация в handler/telegram — Dependency Inversion.
type ReminderSender interface {
	Send(chatID int64, text string) error
}

// SubscriptionReminderService — бизнес-логика напоминаний.
type SubscriptionReminderService struct {
	remna  remnawave.RemnawaveClient
	sender ReminderSender
	logger *zap.Logger
	now    func() time.Time
}

func New(
	remna remnawave.RemnawaveClient,
	sender ReminderSender,
	logger *zap.Logger,
) *SubscriptionReminderService {
	return &SubscriptionReminderService{
		remna:  remna,
		sender: sender,
		logger: logger,
		now:    time.Now,
	}
}

// RunDay — запускает цикл проверки раз в сутки.
func (s *SubscriptionReminderService) RunDay(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(checkPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *SubscriptionReminderService) runOnce(ctx context.Context) {
	now := s.now()

	users, err := s.remna.GetAllUsers(ctx)
	if err != nil {
		s.logger.Error("получение пользователей", zap.Error(err))

		return
	}

	for _, u := range users {
		if ctx.Err() != nil {
			return
		}

		if u.ExpireAt == nil || u.Status != activeStatus {
			continue
		}

		chatID, err := strconv.ParseInt(u.Username, 10, 64)
		if err != nil {
			var numErr *strconv.NumError
			if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrSyntax) {
				continue
			}

			s.logger.Error(
				"парсинг chatID",
				zap.String("uuid", u.UUID),
				zap.Error(err),
			)

			continue
		}

		remaining := u.ExpireAt.Sub(now)
		if remaining > reminderWindow || remaining <= 0 {
			continue
		}

		msg := fmt.Sprintf(
			"Ваша подписка заканчивается %s\n\nОплатите продление, чтобы не потерять доступ.",
			u.ExpireAt.Format("02.01.2006"),
		)

		if err := s.sender.Send(chatID, msg); err != nil {
			s.logger.Error("отправка напоминания", zap.String("uuid", u.UUID), zap.Error(err))

			continue
		}

		s.logger.Info("напоминание отправлено", zap.Int64("chat_id", chatID))
	}
}
