// юкасса добавлена для примера
// ukassa/ukassa.go
package ukassa

import (
	"ProxyMaster_v2/internal/domain"
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	shopID     string
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

func NewClient(shopID, secretKey string) *Client {
	return &Client{
		shopID:    shopID,
		secretKey: secretKey,
		baseURL:   "хз какой, для примера",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

//
//
//
// todo Надо будет сделать PaymentMethod и Currency общим для и юкассы и платеги//я устал...22 09
// todo Надо будет сделать PaymentMethod и Currency общим для и юкассы и платеги//я устал...22 09
// todo Надо будет сделать PaymentMethod и Currency общим для и юкассы и платеги//я устал...22 09
// todo Надо будет сделать PaymentMethod и Currency общим для и юкассы и платеги//я устал...22 09
// todo Надо будет сделать PaymentMethod и Currency общим для и юкассы и платеги//я устал...22 09
// и все на интерфейсы повесить

// CreateTransaction - создает платеж в UKassa
func (c *Client) CreateTransaction(ctx context.Context, paymentMethod PaymentMethod, amount int, currency Currency, description string, payload string) (URL string, err error) {
	_ = ctx

	// Заглушка
	return fmt.Sprintf("%s/payment/test_456", c.baseURL), "ukassa_payment_123", nil
}

// CheckStatus - проверяет статус платежа
func (c *Client) CheckStatus(ctx context.Context, transactionID string) (domain.PaymentStatus, error) {
	_ = ctx
	_ = transactionID

	// Заглушка
	return domain.PaymentStatusPending, nil
}

// GetTransactionInfo - получает информацию о платеже
func (c *Client) GetTransactionInfo(ctx context.Context, transactionID string) (domain.TransactionInfo, error) {
	_ = ctx
	_ = transactionID

	// Заглушка
	response := &CreatePaymentResponse{
		ID:     transactionID,
		Status: "pending",
		Amount: map[string]interface{}{
			"value":    "100.50",
			"currency": "RUB",
		},
		Description: "Тестовый платеж",
		Paid:        false,
	}

	return &TransactionInfo{response: response}, nil
}
