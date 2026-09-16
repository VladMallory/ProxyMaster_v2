package paymentsvc

import (
	"context"
	"log/slog"
	"time"

	paymentdomain "github.com/VladMallory/ProxyMaster_v2/internal/payment/domain"
)

type PaymentService interface {
	CreateInvoice(
		ctx context.Context,
		userID string,
		amount int,
	) (invoiceID, payURL string, err error)
	CheckStatus(ctx context.Context, invoiceID string) (success bool, err error)
}

type SubscriptionExtender interface {
	ExtendSubscription(ctx context.Context, userID string, days int) error
}

type ResultNotifier interface {
	NotifySuccess(userID string, months int)
	NotifyTimeout(userID string)
}

const (
	pollInterval = 10 * time.Second
	pollTimeout  = 20 * time.Minute
)

type Service struct {
	paymentService PaymentService
	extender       SubscriptionExtender
	notifier       ResultNotifier
	tariffs        []paymentdomain.Tariff
}

func NewPayment(
	paymentService PaymentService,
	extender SubscriptionExtender,
	notifier ResultNotifier,
	tariffs []paymentdomain.Tariff,
) *Service {
	return &Service{
		paymentService: paymentService,
		extender:       extender,
		notifier:       notifier,
		tariffs:        tariffs,
	}
}

func (s *Service) Tariffs() []paymentdomain.Tariff {
	return s.tariffs
}

func (s *Service) CreatePayment(ctx context.Context, userID string, tariffIdx int) (string, error) {
	tariff := s.tariffs[tariffIdx]
	amount := tariff.PriceRub * 100

	invoiceID, payURL, err := s.paymentService.CreateInvoice(ctx, userID, amount)
	if err != nil {
		return "", err
	}

	// Имеет свой контекст
	go s.watchPayment(context.WithoutCancel(ctx), invoiceID, userID, tariff.Months)

	return payURL, nil
}

func (s *Service) watchPayment(
	ctx context.Context,
	invoiceID,
	userID string,
	months int,
) {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.notifier.NotifyTimeout(userID)

			return

		case <-ticker.C:
			ok, err := s.paymentService.CheckStatus(ctx, invoiceID)
			if err != nil {
				continue
			}

			// Если платеж еще не ok, то заново идем в цикл
			if !ok {
				continue
			}

			// Если платеж ok:
			if err := s.extender.ExtendSubscription(ctx, userID, months); err != nil {
				slog.Error("оплата прошла, продление не удалось", "user", userID, "err", err)

				return
			}

			s.notifier.NotifySuccess(userID, months)

			return
		}
	}
}
