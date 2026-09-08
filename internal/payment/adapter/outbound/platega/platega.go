package platega

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://app.platega.io"
)

// PaymentMethod код способа оплаты
type PaymentMethod int

const (
	PaymentMethodSBP           PaymentMethod = 2  // СБП (QR-код)
	PaymentMethodERIP          PaymentMethod = 3  // ЕРИП
	PaymentMethodCardAcquiring PaymentMethod = 11 // Карточный эквайринг
	PaymentMethodInternational PaymentMethod = 12 // Международная оплата
	PaymentMethodCrypto        PaymentMethod = 13 // Криптовалюта
	PaymentMethodSberpay       PaymentMethod = 14 // Sberpay
)

// TransactionStatus статус транзакции
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "PENDING"
	StatusConfirmed  TransactionStatus = "CONFIRMED"
	StatusCanceled   TransactionStatus = "CANCELED"
	StatusChargeback TransactionStatus = "CHARGEBACKED"
)

// PaymentDetails детали платежа
type PaymentDetails struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// CreateTransactionRequest запрос на создание транзакции
type CreateTransactionRequest struct {
	PaymentMethod  *PaymentMethod `json:"paymentMethod,omitempty"`
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description,omitempty"`
	Return         string         `json:"return,omitempty"`
	FailedURL      string         `json:"failedUrl,omitempty"`
	Payload        string         `json:"payload,omitempty"`
	Metadata       *Metadata      `json:"metadata,omitempty"`
}

// Metadata метаданные плательщика (нужны для антрафрод)
type Metadata struct {
	UserID   string `json:"userId,omitempty"`
	UserName string `json:"userName,omitempty"`
}

// CreateTransactionResponse ответ на создание транзакции
type CreateTransactionResponse struct {
	TransactionID string `json:"transactionId"`
	Redirect      string `json:"redirect"`
	URL           string `json:"url,omitempty"`
	ExpiresIn     string `json:"expiresIn,omitempty"`
}

// TransactionStatusResponse ответ со статусом транзакции
type TransactionStatusResponse struct {
	ID            string            `json:"id"`
	Status        TransactionStatus `json:"status"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	PaymentMethod int               `json:"paymentMethod"`
	Description   string            `json:"description"`
	Payload       string            `json:"payload,omitempty"`
	Comission     float64           `json:"comission,omitempty"` // опечатка в API Platega
	MerchantID    string            `json:"mechantId,omitempty"` // опечатка в API Platega
	CreatedAt     string            `json:"createdAt,omitempty"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
}

// CallbackData данные callback-уведомления
type CallbackData struct {
	ID            string            `json:"id"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Status        TransactionStatus `json:"status"`
	PaymentMethod int               `json:"paymentMethod"`
	Payload       string            `json:"payload,omitempty"`
}

// PlategaClient клиент для работы с Platega API
type PlategaClient struct {
	merchantID string
	secret     string
	baseURL    string
	httpClient *http.Client
}

// NewClient создаёт новый Platega-клиент
func NewClient(merchantID, secret string) *PlategaClient {
	return &PlategaClient{
		merchantID: merchantID,
		secret:     secret,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL переопределяет базовый URL (для тестов)
func (c *PlategaClient) SetBaseURL(url string) {
	c.baseURL = url
}

// doRequest выполняет HTTP-запрос к API
func (c *PlategaClient) doRequest(method, path string, body any) ([]byte, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}

		reqBody = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-MerchantId", c.merchantID)
	req.Header.Set("X-Secret", c.secret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("platega API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateTransaction создаёт транзакцию с указанным способом оплаты
func (c *PlategaClient) CreateTransaction(
	req CreateTransactionRequest,
) (*CreateTransactionResponse, error) {
	respBody, err := c.doRequest(http.MethodPost, "/v2/transaction/process", req)
	if err != nil {
		return nil, err
	}

	var result CreateTransactionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// CreatePaymentLink создаёт платёжную ссылку (плательщик сам выбирает метод)
func (c *PlategaClient) CreatePaymentLink(
	details PaymentDetails,
	description, returnURL, failedURL, payload string,
) (*CreateTransactionResponse, error) {
	req := CreateTransactionRequest{
		PaymentDetails: details,
		Description:    description,
		Return:         returnURL,
		FailedURL:      failedURL,
		Payload:        payload,
	}

	return c.CreateTransaction(req)
}

// GetTransactionStatus получает статус транзакции по ID
func (c *PlategaClient) GetTransactionStatus(
	transactionID string,
) (*TransactionStatusResponse, error) {
	respBody, err := c.doRequest(http.MethodGet, "/v2/transaction/"+transactionID, nil)
	if err != nil {
		return nil, err
	}

	var result TransactionStatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// VerifyCallback проверяет подлинность callback-уведомления
func (c *PlategaClient) VerifyCallback(headers map[string]string) bool {
	merchantID := headers["X-Merchantid"]
	secret := headers["X-Secret"]
	return merchantID == c.merchantID && secret == c.secret
}

// ParseCallback парсит тело callback-уведомления
func ParseCallback(body []byte) (*CallbackData, error) {
	var result CallbackData
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse callback: %w", err)
	}
	return &result, nil
}
