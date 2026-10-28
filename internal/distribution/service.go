package distribution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	ErrEmptyText     = errors.New("пустой текст рассылки")
	ErrBusy          = errors.New("рассылка уже идёт")
	ErrBlockedByUser = errors.New("бот не может написать пользователю")
)

type Recipients interface {
	All(ctx context.Context) ([]int64, error)
}

type Sender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

type Report struct {
	Total   int
	Sent    int
	Blocked int
	Failed  int
}

type Service struct {
	recipients Recipients
	sender     Sender
	log        *zap.Logger

	mu   sync.Mutex
	busy bool
}

func NewService(recipients Recipients, sender Sender, log *zap.Logger) *Service {
	return &Service{
		recipients: recipients,
		sender:     sender,
		log:        log,
	}
}

func (s *Service) Broadcast(ctx context.Context, text string) (Report, error) {
	if strings.TrimSpace(text) == "" {
		return Report{}, ErrEmptyText
	}

	if !s.acquire() {
		return Report{}, ErrBusy
	}

	defer s.release()

	ids, err := s.recipients.All(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("получить получателей: %w", err)
	}

	report := Report{Total: len(ids)}
	for _, id := range ids {
		switch err := s.sender.Send(ctx, id, text); {
		case err == nil:
			report.Sent++
		case errors.Is(err, ErrBlockedByUser):
			report.Blocked++
		default:
			report.Failed++
			s.log.Warn("рассылка: сообщение не ушло",
				zap.Int64("telegram_id", id),
				zap.Error(err))
		}
	}

	return report, nil
}

func (s *Service) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.busy {
		return false
	}

	s.busy = true

	return true
}

func (s *Service) release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.busy = false
}
