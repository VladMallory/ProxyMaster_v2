// Package domain описание контрактов (интерфейсов)
// для работы с платежными системами
package domain

import "context"

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
)

// PaymentGateway Общий интерфейс для всех платежных систем
type PaymentGateway interface {
	CreateTransaction(ctx context.Context, amount float64, orderID string) (paymentURL, externalID string, err error)
	CheckStatus(ctx context.Context, transactionID string) (PaymentStatus, error)
	GetTransactionInfo(ctx context.Context, transactionID string) (TransactionInfo, error)
}

// TransactionInfo Общий интерфейс для информации о транзакции
type TransactionInfo interface {
	GetID() string
	GetAmount() float64
	GetStatus() string
	GetRawResponse() interface{}
}
