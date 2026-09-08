package paymentservice

import (
	"context"
	"log/slog"
	"time"
)

type PaymentGateway interface {
	CreatePayment(
		ctx context.Context,
		userID string,
		amount int,
	) (paymentID, payURL string, err error)
	CheckStatus(ctx context.Context, paymentID string) (paid bool, err error)
}

type PaymentObserver interface {
	OnPaymentConfirmed(ctx context.Context, userID string, amount int) error
}

type PaymentService struct {
	ctx       context.Context
	cancel    context.CancelFunc
	timeout   time.Duration
	gateway   PaymentGateway
	observers []PaymentObserver
	pollEvery time.Duration
}

func NewPaymentService(
	ctx context.Context,
	gateway PaymentGateway,
	observers ...PaymentObserver,
) *PaymentService {
	serviceCtx, cancel := context.WithCancel(ctx)

	return &PaymentService{
		ctx:       serviceCtx,
		cancel:    cancel,
		timeout:   20 * time.Minute,
		gateway:   gateway,
		observers: observers,
		pollEvery: 10 * time.Second,
	}
}

// StartTopUp создаёт платёж и запускает фоновое наблюдение за его статусом.
// Возвращает ссылку на оплату сразу не дожидаясь подтверждения.
func (s *PaymentService) StartTopUp(
	ctx context.Context,
	userID string,
	amount int,
) (string, error) {
	paymentID, payURL, err := s.gateway.CreatePayment(ctx, userID, amount)
	if err != nil {
		return "", err
	}

	go s.watch(ctx, paymentID, userID, amount)

	return payURL, nil
}

// TODO: разобраться с контекстом.
func (s *PaymentService) watch(ctx context.Context, paymentID, userID string, amount int) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("оплата не подтверждена за отведённое время", "payment_id", paymentID)

			return

		case <-ticker.C:
			paid, err := s.gateway.CheckStatus(ctx, paymentID)
			if err != nil {
				slog.Warn("ошибка проверки статуса оплаты", "payment_id", paymentID, "error", err)

				continue
			}
			if paid {
				s.notify(ctx, userID, amount)

				return
			}
		}
	}
}

func (s *PaymentService) notify(ctx context.Context, userID string, amount int) {
	for _, obs := range s.observers {
		if err := obs.OnPaymentConfirmed(ctx, userID, amount); err != nil {
			slog.Error("observer упал при подтверждении оплаты", "error", err)
		}
	}
}
