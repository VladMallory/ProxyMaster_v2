package youkassa2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// YooKassaPaymentStatusResponse ответ ЮKassa при проверке платежа.
// Paid == true и Status == "succeeded" означают, что оплата реально прошла.
type YooKassaPaymentStatusResponse struct {
	ID       string           `json:"id"`
	Status   string           `json:"status"`
	Paid     bool             `json:"paid"`
	Metadata YooKassaMetadata `json:"metadata"`
}

// YooKassaCreatePaymentRequest запрос, который мы отправляем в ЮKassa.
type YooKassaCreatePaymentRequest struct {
	Amount       YooKassaAmount       `json:"amount"`
	Capture      bool                 `json:"capture"`
	Confirmation YooKassaConfirmation `json:"confirmation"`
	Description  string               `json:"description"`
	Metadata     YooKassaMetadata     `json:"metadata"`
}

// YooKassaAmount сумма платежа.
// Value у ЮKassa должен быть строкой, например "500.00".
type YooKassaAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// YooKassaConfirmation описывает, как ЮKassa подтвердит платеж.
// Type redirect значит, что пользователь перейдет на страницу оплаты ЮKassa.
type YooKassaConfirmation struct {
	Type      string `json:"type"`
	ReturnURL string `json:"return_url"`
}

// YooKassaMetadata наши внутренние данные.
// Username кладем сюда, чтобы потом в webhook понять, кто оплатил.
type YooKassaMetadata struct {
	Username string `json:"username"`
	Days     string `json:"days"`
}

// YooKassaCreatePaymentResponse ответ ЮKassa после создания платежа.
type YooKassaCreatePaymentResponse struct {
	ID           string                         `json:"id"`
	Status       string                         `json:"status"`
	Confirmation YooKassaPaymentConfirmationURL `json:"confirmation"`
}

// YooKassaPaymentConfirmationURL содержит ссылку на оплату.
type YooKassaPaymentConfirmationURL struct {
	Type            string `json:"type"`
	ConfirmationURL string `json:"confirmation_url"`
}

// YooKassaClient отвечает только за работу с ЮKassa.
type YooKassaClient struct {
	ShopID     string
	SecretKey  string
	HTTPClient *http.Client
}

func (c *YooKassaClient) CreatePayment(
	username string,
	amount int64,
) (*YooKassaCreatePaymentResponse, error) {
	days := int(amount / 200 * 30)

	paymentRequest := YooKassaCreatePaymentRequest{
		Amount: YooKassaAmount{
			Value:    formatRubAmount(amount),
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: YooKassaConfirmation{
			Type:      "redirect",
			ReturnURL: "https://google.com/",
		},
		Description: "оплата доступна для пользователя" + username,
		Metadata: YooKassaMetadata{
			Username: username,
			Days:     fmt.Sprintf("%d", days),
		},
	}

	body, err := json.Marshal(paymentRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.yookassa.ru/v3/payments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Idempotence-Key", uuid.NewString())

	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("Authorization", "Basic "+basicAuth(c.ShopID, c.SecretKey))

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"юкасса вернула ошибочный статус %d",
			resp.StatusCode,
		)
	}

	var paymentResponse YooKassaCreatePaymentResponse

	err = json.NewDecoder(resp.Body).Decode(&paymentResponse)
	if err != nil {
		return nil, err
	}

	return &paymentResponse, nil
}

func (c *YooKassaClient) GetPayment(
	ctx context.Context,
	paymentID string,
) (*YooKassaPaymentStatusResponse, error) {
	if paymentID == "" {
		return nil, fmt.Errorf("пустой paymentID")
	}

	// Создаем запрос именно с context, чтобы сервер мог отменить запрос при таймауте.
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.yookassa.ru/v3/payments/"+paymentID,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// ЮKassa использует Basic Auth: shopID:secretKey.
	req.Header.Set("Authorization", "Basic "+basicAuth(c.ShopID, c.SecretKey))
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Отправляем запрос в ЮKassa, а не верим данным от пользователя.
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"юкасса вернула ошибочный статус проверки %d",
			resp.StatusCode,
		)
	}

	var payment YooKassaPaymentStatusResponse
	// Декодируем JSON-ответ ЮKassa в нашу структуру.
	err = json.NewDecoder(resp.Body).Decode(&payment)
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func basicAuth(shopID, secretKey string) string {
	authvalue := shopID + ":" + secretKey

	return base64.StdEncoding.EncodeToString([]byte(authvalue))
}

func formatRubAmount(amount int64) string {
	return fmt.Sprintf("%d.00", amount)
}
