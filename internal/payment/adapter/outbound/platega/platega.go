package platega

import (
	"context"
	"fmt"
)

// Gateway — заглушка платёжного шлюза Platega.
// Реализует paymentcase.PaymentGateway.
type Gateway struct{}

func New() *Gateway {
	return &Gateway{}
}

// CreatePayment создаёт платёж и возвращает ID и URL для оплаты.
// Заглушка: всегда возвращает фиктивные данные.
func (g *Gateway) CreatePayment(
	ctx context.Context,
	userID string,
	amount int,
) (paymentID, payURL string, err error) {
	return fmt.Sprintf("platega_stub_%s_%d", userID, amount),
		"https://example.com/pay/stub",
		nil
}

// CheckStatus проверяет статус оплаты по ID.
// Заглушка: всегда возвращает false.
func (g *Gateway) CheckStatus(ctx context.Context, paymentID string) (paid bool, err error) {
	return false, nil
}
