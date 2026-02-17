// Package platega реализация клиента с платежной системной platega.
package platega

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

// NewClient создает новый экземпляр клиента Platega.
// func NewClient(apiKey string, l logger.Logger) *Client {
// 	return &Client{
// 		baseURL: "https://app.platega.io",
// 		apiKey:  apiKey,
// 		httpClient: &http.Client{
// 			Timeout: 30 * time.Second,
// 		},
// 		logger: l,
// 	}
// }

// logDuration логирует время выполнения метода.
func (c *Client) logDuration(method string) func() {
	start := time.Now()

	return func() {
		c.logger.Info("вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

// CreateTransaction - создает новую транзакцию в Platega
//
//	paymentMethod - метод оплаты, типа PaymentMethod(RUB, USDT, etc...)
//	amount        - цена услуги int
//	currency      - валюта, типа  Currency
//	description   - "Оплата мешков картошки клиенту №293" string
//	payload       - инфа которую можно дополнительно оставить (как я понял) ни на что не влияет,   можно "" string
//
//nolint:cyclop, funlen
func (c *Client) CreateTransaction(
	ctx context.Context,
	paymentMethod PaymentMethod,
	amount int,
	currency Currency,
	description, payload string,
) (URL, ID string, err error) {
	defer c.logDuration("CreateTransaction")

	merchantID := os.Getenv("PLATEGA_MERCHANT_ID")
	if merchantID == "" {
		return "", "", fmt.Errorf("platega.CreateTransaction: MERCHANT_ID не установлен в .env")
	}

	plategaAPIKey := c.apiKey
	if plategaAPIKey == "" {
		plategaAPIKey = os.Getenv("PLATEGA_API_KEY")
	}

	if plategaAPIKey == "" {
		return "",
			"",
			fmt.Errorf("platega.CreateTransaction: PLATEGA_API_KEY не установлен (ни в клиенте, ни в .env)") //nolint:golines
	}

	// сборка. URL
	plategaBaseURL := c.baseURL
	if plategaBaseURL == "" {
		plategaBaseURL = os.Getenv("PLATEGA_BASE_URL")
	}

	if plategaBaseURL == "" {
		return "", "", fmt.Errorf("platega.CreateTransaction: PLATEGA_BASE_URL не установлен (ни в клиенте, ни в .env)") //nolint:lll,golines
	}

	plategaTotalURL := plategaBaseURL + "/transaction/process"

	// сборка реквеста
	reqBody := CreateTransactionRequest{
		PaymentMethod: int(paymentMethod),
		PaymentDetails: PaymentDetails{
			Amount:   amount,
			Currency: string(currency),
		},
		Description: description,
		ReturnURL:   "https://google.com/success", // TODO: уточнить значение URL успеха
		FailedURL:   "https://google.com/fail",    // TODO: уточнить значение URL ошибки
		Payload:     payload,
	}

	// маршалинг реквеста
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("platega.CreateTransaction: ошибка маршалинга: %w", err)
	}

	// запрос к апи platega
	req, err := http.NewRequestWithContext(ctx, "POST", plategaTotalURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("platega.CreateTransaction: ошибка создания запроса: %w", err)
	}

	req.Header.Set("X-MerchantId", merchantID)
	req.Header.Set("X-Secret", plategaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("platega.CreateTransaction: ошибка получения ответа: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Error(
				"ошибка про закрытии тела ответа",
				logger.Field{Key: "method", Value: "CreateTransaction"},
				logger.Field{Key: "err_msg", Value: err},
			)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("platega.CreateTransaction: ошибка чтения тела ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("platega.CreateTransaction: код статуса: %v\nОшибка: %s", resp.StatusCode, string(respBody)) //nolint:lll,golines
	}

	var CreateTransactionResponse CreateTransactionResponse

	err = json.Unmarshal(respBody, &CreateTransactionResponse)
	if err != nil {
		return "", "", fmt.Errorf("platega.CreateTransaction: ошибка анмаршалинга ответа: %w", err)
	}

	URL = CreateTransactionResponse.Redirect
	ID = CreateTransactionResponse.TransactionID

	return URL, ID, nil
}

// CheckStatus проверяет статус транзакции.
func (c *Client) CheckStatus(ctx context.Context, transactionID string) (domain.PaymentStatus, error) { //nolint:golines
	defer c.logDuration("CheckStatus")

	// Получаем инфу о транзакции, чтобы узнать статус
	info, err := c.GetTransactionInfo(ctx, transactionID)
	if err != nil {
		return "", err
	}

	status := info.GetStatus()

	switch status {
	case "success", "paid":
		return domain.PaymentStatusSuccess, nil
	case "pending", "process":
		return domain.PaymentStatusPending, nil
	case "failed", "error":
		return domain.PaymentStatusFailed, nil
	default:
		return domain.PaymentStatusPending, nil
	}
}

// GetTransactionInfo получает информацию о транзакции.
func (c *Client) GetTransactionInfo(ctx context.Context, transactionID string) (domain.TransactionInfo, error) { //nolint:lll,golines
	defer c.logDuration("GetTransactionInfo")

	plategaBaseURL := c.baseURL
	if plategaBaseURL == "" {
		plategaBaseURL = os.Getenv("PLATEGA_BASE_URL")
	}

	// Предполагаемый эндпоинт для проверки статуса (нужно уточнить в документации Platega)
	// Обычно это GET /transaction/{id} или POST /transaction/status
	// Используем GET для примера, если документации нет под рукой
	url := fmt.Sprintf("%s/transaction/%s", plategaBaseURL, transactionID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("platega.GetTransactionInfo: ошибка создания запроса: %w", err)
	}

	req.Header.Set("X-MerchantId", os.Getenv("PLATEGA_MERCHANT_ID"))
	req.Header.Set("X-Secret", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platega.GetTransactionInfo: ошибка выполнения запроса: %w", err)
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			return
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("platega.GetTransactionInfo: ошибка чтения ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platega.GetTransactionInfo: статус %d, ответ: %s", resp.StatusCode, string(body)) //nolint:lll,golines
	}

	var txInfo TransactionInfoResponse
	if err := json.Unmarshal(body, &txInfo); err != nil {
		return nil, fmt.Errorf("platega.GetTransactionInfo: ошибка парсинга JSON: %w", err)
	}

	return &txInfo, nil
}

// === Методы для реализации интерфейса domain.TransactionInfo ===

func (t *TransactionInfoResponse) GetID() string {
	return t.ID
}

func (t *TransactionInfoResponse) GetAmount() float64 {
	return t.PaymentDetails.Amount
}

func (t *TransactionInfoResponse) GetStatus() string {
	return t.Status
}

func (t *TransactionInfoResponse) GetRawResponse() any {
	return t
}
