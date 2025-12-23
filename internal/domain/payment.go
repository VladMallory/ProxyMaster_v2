// Предназначение: Описание контрактов (интерфейсов) для работы с платежными системами
package domain

import "context"

// PaymentStatus - статус платежа (собственный тип, чтобы не зависеть от строк провайдера)
type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending" // Ожидает оплаты
	PaymentStatusSuccess PaymentStatus = "success" // Оплачен
	PaymentStatusFailed  PaymentStatus = "failed"  // Ошибка
)

// PaymentGateway - абстракция любой платежной системы.
// Позволяет бизнес-логике не зависеть от конкретного провайдера (Platega, Stripe и т.д.).
type PaymentGateway interface {
	// CreatePayment создает ссылку на оплату.
	// Принимает: контекст, сумму, ID заказа.
	// Возвращает: URL для оплаты, ID платежа во внешней системе, ошибку.
	CreatePayment(ctx context.Context, amount float64, orderID string) (paymentURL string, externalID string, err error)

	// CheckStatus проверяет статус платежа по его ID во внешней системе.
	CheckStatus(ctx context.Context, externalID string) (PaymentStatus, error)
}
