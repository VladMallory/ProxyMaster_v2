package paymentsvc

import (
	"context"
	"errors"
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

// ResultNotifier уведомляет клиента об итоге оплаты.
type ResultNotifier interface {
	NotifySuccess(userID string)
	NotifyTimeout(userID string)
}

const (
	pollInterval = time.Second
	pollTimeout  = 20 * time.Minute
)

type Service struct {
	paymentService PaymentService
	notifier       ResultNotifier
	tariffs        []paymentdomain.Tariff
}

func NewPayment(
	paymentService PaymentService,
	notifier ResultNotifier,
	tariffs []paymentdomain.Tariff,
) *Service {
	return &Service{
		paymentService: paymentService,
		notifier:       notifier,
		tariffs:        tariffs,
	}
}

func (s *Service) Tariffs() []paymentdomain.Tariff {
	return s.tariffs
}

// CreatePayment создает оплату и проверяет ее.
func (s *Service) CreatePayment(
	ctx context.Context,
	userID string,
	amountRub int,
	onPaid func(ctx context.Context, paymentID string) error,
) (string, error) {
	if userID == "" || amountRub <= 0 {
		return "", errors.New("invalid payment arguments")
	}

	// Провайдер ждёт копейки, поэтому рубли в копейки.
	amount := amountRub * 100

	invoiceID, payURL, err := s.paymentService.CreateInvoice(ctx, userID, amount)
	if err != nil {
		return "", err
	}

	go s.watchPayment(context.WithoutCancel(ctx), invoiceID, userID, onPaid)

	return payURL, nil
}

// watchPayment опрашивает статус, если успешно, вызывает успех.
func (s *Service) watchPayment(
	ctx context.Context,
	invoiceID, userID string,
	onPaid func(ctx context.Context, paymentID string) error,
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

			if !ok {
				continue
			}

			if err := onPaid(ctx, invoiceID); err != nil {
				return
			}

			s.notifier.NotifySuccess(userID)

			return
		}
	}
}
