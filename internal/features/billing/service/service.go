package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
)

// PaymentGateway — контракт платёжной системы.
type PaymentGateway interface {
	CreateTransaction(ctx context.Context, amount float64, orderID string) (paymentURL, externalID string, err error)
	CheckStatus(ctx context.Context, transactionID string) (domain.PaymentStatus, error)
	GetTransactionInfo(ctx context.Context, transactionID string) (TransactionInfo, error)
}

// TransactionInfo — информация о транзакции.
type TransactionInfo interface {
	GetID() string
	GetAmount() float64
	GetStatus() string
	GetRawResponse() any
}

type Purpose string

const (
	PurposeAddDevice     Purpose = "add_device"
	PurposePrepayDevices Purpose = "prepay_devices"
	PurposeResetTraffic  Purpose = "reset_traffic"
	PurposeSubscription  Purpose = "subscription"
)

type paymentData struct {
	purpose Purpose
	amount  int
}

// Service — бизнес-логика платежей.
type Service struct {
	gateway  PaymentGateway
	payments sync.Map // transactionID → paymentData
}

func New(gateway PaymentGateway) *Service {
	return &Service{gateway: gateway}
}

// CreatePayment создаёт платёж и сохраняет его цель.
func (s *Service) CreatePayment(
	ctx context.Context,
	userID string,
	amount int,
	purpose Purpose,
) (paymentURL, transactionID string, err error) {
	orderID := fmt.Sprintf("tg_%s_%d_%d", userID, amount, time.Now().UnixNano())

	url, id, err := s.gateway.CreateTransaction(ctx, float64(amount), orderID)
	if err != nil {
		return "", "", err
	}

	s.payments.Store(id, paymentData{purpose: purpose, amount: amount})

	return url, id, nil
}

// CheckPayment проверяет статус платежа.
func (s *Service) CheckPayment(
	ctx context.Context,
	transactionID string,
) (domain.PaymentStatus, error) {
	return s.gateway.CheckStatus(ctx, transactionID)
}

// GetPaymentInfo возвращает детальную информацию о транзакции.
func (s *Service) GetPaymentInfo(
	ctx context.Context,
	transactionID string,
) (TransactionInfo, error) {
	return s.gateway.GetTransactionInfo(ctx, transactionID)
}

// ConsumePaymentPurpose читает и удаляет сохранённую цель платежа.
func (s *Service) ConsumePaymentPurpose(transactionID string) (
	purpose Purpose,
	amount int, ok bool,
) {
	v, loaded := s.payments.Load(transactionID)
	if !loaded {
		return "", 0, false
	}
	s.payments.Delete(transactionID)
	data := v.(paymentData)

	return data.purpose, data.amount, true
}
