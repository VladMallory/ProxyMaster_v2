package payment

import (
	"context"
	"errors"
)

const (
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusCanceled  PaymentStatus = "canceled"
	PaymentStatusPending   PaymentStatus = "pending"
)

type PaymentStatus string

// PaymentInfo минимальная информация о платиже.
type PaymentInfo struct {
	ID       string
	Status   PaymentStatus
	Paid     bool
	Username string
}

type PaymentProvider interface {
	GetPayment(ctx context.Context, paymentID string) (*PaymentInfo, error)
}

type PaymentRepository interface {
	SavePayment(ctx context.Context, payment *PaymentInfo) error
}

type PaymentService struct {
	provider PaymentProvider
	repo     PaymentRepository
}

func NewPaymentService(provider PaymentProvider, repo PaymentRepository) *PaymentService {
	return &PaymentService{
		provider: provider,
		repo:     repo,
	}
}

func (s *PaymentService) CheckPayment(ctx context.Context, paymentID string) (bool, error) {
	if paymentID == "" {
		return false, errors.New("пустой id оплаты")
	}

	payment, err := s.provider.GetPayment(ctx, paymentID)
	if err != nil {
		return false, err
	}

	// Сохраняем текущий статус платежа.
	// Это полезно, чтобы потом видеть историю и не дергать ЮKassa каждый раз.
	err = s.repo.SavePayment(ctx, payment)
	if err != nil {
		return false, err
	}

	if payment.Status == PaymentStatusSucceeded && payment.Paid {
		return true, nil
	}

	return false, nil
}
