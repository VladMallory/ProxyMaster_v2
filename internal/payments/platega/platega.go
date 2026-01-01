// Package platega реализация клиента с платежной системной platega.
package platega

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// NewClient создает новый экземпляр клиента Platega.
func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: "https://app.platega.io",
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateTransaction - создает новую транзакцию в Platega
//
//	paymentMethod - метод оплаты, типа PaymentMethod(RUB, USDT, etc...)
//	amount        - цена услуги int
//	currency      - валюта, типа  Currency
//	description   - "Оплата мешков картошки клиенту №293" string
//	payload       - инфа которую можно дополнительно оставить (как я понял) ни на что не влияет,   можно "" string
func (c *Client) CreateTransaction(
	ctx context.Context,
	paymentMethod PaymentMethod,
	amount int,
	currency Currency,
	description, payload string,
) (URL string, err error) {
	merchantID := os.Getenv("PLATEGA_MERCHANT_ID")
	if merchantID == "" {
		return "", fmt.Errorf("platega.CreateTransaction: MERCHANT_ID не установлен в .env")
	}

	plategaAPIKey := c.apiKey
	if plategaAPIKey == "" {
		plategaAPIKey = os.Getenv("PLATEGA_API_KEY")
	}
	if plategaAPIKey == "" {
		return "", fmt.Errorf("platega.CreateTransaction: PLATEGA_API_KEY не установлен (ни в клиенте, ни в .env)")
	}

	// сборка. URL
	plategaBaseURL := c.baseURL
	if plategaBaseURL == "" {
		plategaBaseURL = os.Getenv("PLATEGA_BASE_URL")
	}
	if plategaBaseURL == "" {
		return "", fmt.Errorf("platega.CreateTransaction: PLATEGA_BASE_URL не установлен (ни в клиенте, ни в .env)")
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
		return "", fmt.Errorf("platega.CreateTransaction: ошибка маршалинга: %w", err)
	}

	// запрос к апи platega
	req, err := http.NewRequestWithContext(ctx, "POST", plategaTotalURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("platega.CreateTransaction: ошибка создания запроса: %w", err)
	}
	req.Header.Set("X-MerchantId", merchantID)
	req.Header.Set("X-Secret", plategaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("platega.CreateTransaction: ошибка получения ответа: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("platega.CreateTransaction: ошибка при закрытии тела ответа: %v", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("platega.CreateTransaction: ошибка чтения тела ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("platega.CreateTransaction: код статуса: %v\nОшибка: %s", resp.StatusCode, string(respBody))
	}

	var CreateTransactionResponse CreateTransactionResponse
	err = json.Unmarshal(respBody, &CreateTransactionResponse)
	if err != nil {
		return "", fmt.Errorf("platega.CreateTransaction: ошибка анмаршалинга ответа: %w", err)
	}

	URL = CreateTransactionResponse.Redirect
	// ID := CreateTransactionResponse.TransactionID

	return URL, nil
}
