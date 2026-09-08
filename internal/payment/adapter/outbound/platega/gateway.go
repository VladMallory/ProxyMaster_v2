package platega

import (
	"context"

	paymentservice "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
)

// Gateway реализует paymentservice.PaymentGateway поверх PlategaClient.
// PlategaClient остаётся тонкой обёрткой над HTTP API и ничего не знает
// про тип paymentservice.PaymentGateway — эту связь делает только Gateway.
type Gateway struct {
	client    *PlategaClient
	returnURL string
	failedURL string
	currency  string
}

// NewGateway создаёт Gateway, связывая HTTP-клиент Platega с конфигурацией платёжных URL.
func NewGateway(client *PlategaClient, returnURL, failedURL, currency string) *Gateway {
	return &Gateway{
		client:    client,
		returnURL: returnURL,
		failedURL: failedURL,
		currency:  currency,
	}
}

// Compile-time проверка: Gateway точно реализует PaymentGateway.
var _ paymentservice.PaymentGateway = (*Gateway)(nil)

// CreatePayment создаёт платёжную ссылку через Platega API.
// Возвращает ID транзакции и URL для редиректа пользователя.
func (g *Gateway) CreatePayment(
	ctx context.Context,
	userID string,
	amount int,
) (paymentID, payURL string, err error) {
	resp, err := g.client.CreatePaymentLink(
		PaymentDetails{Amount: float64(amount), Currency: g.currency},
		"Продление подписки ProxyMaster",
		g.returnURL,
		g.failedURL,
		userID, // payload — по нему потом свяжете callback с телеграм-юзером
	)
	if err != nil {
		return "", "", err
	}

	url := resp.URL
	if url == "" {
		url = resp.Redirect
	}

	return resp.TransactionID, url, nil
}

// CheckStatus проверяет, подтверждена ли транзакция по её ID.
func (g *Gateway) CheckStatus(ctx context.Context, paymentID string) (bool, error) {
	status, err := g.client.GetTransactionStatus(paymentID)
	if err != nil {
		return false, err
	}

	return status.Status == StatusConfirmed, nil
}
