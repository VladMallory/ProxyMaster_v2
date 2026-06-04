// Package platega реализация клиента с платежной системной platega.
package platega

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

// NewClient создает новый экземпляр клиента Platega.
// Если параметры пустые — подставляет из переменных окружения.
func NewClient(merchantID, apiKey, returnURL string, l logger.Logger) *Client {
	if merchantID == "" {
		merchantID = strings.TrimSpace(os.Getenv("PLATEGA_MERCHANT_ID"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("PLATEGA_API_KEY"))
	}
	if returnURL == "" {
		returnURL = strings.TrimSpace(os.Getenv("PLATEGA_RETURN_URL"))
	}

	c := &Client{
		baseURL:    "https://app.platega.io",
		merchantID: merchantID,
		apiKey:     apiKey,
		returnURL:  returnURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     l,
	}

	if v := strings.TrimSpace(os.Getenv("PLATEGA_BASE_URL")); v != "" {
		c.baseURL = v
	}

	return c
}

// logDuration логирует время выполнения и ошибку (если была).
func (c *Client) logDuration(method string, err *error) func() {
	start := time.Now()

	return func() {
		if *err != nil {
			c.logger.Error("вызов метода завершен ошибкой",
				logger.Field{Key: "method", Value: method},
				logger.Field{Key: "err", Value: (*err).Error()},
				logger.Field{Key: "duration", Value: time.Since(start)},
			)

			return
		}

		c.logger.Info("вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

// newIdempotenceKey генерирует уникальный ключ идемпотентности для Platega.
func newIdempotenceKey(orderID string) (string, error) {
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return "", fmt.Errorf("platega.newIdempotenceKey: %w", err)
	}

	rndHex := hex.EncodeToString(rnd)

	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return rndHex, nil
	}

	orderID = strings.ReplaceAll(orderID, " ", "_")
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	key := orderID + "-" + rndHex
	if len(key) > 64 {
		key = key[:64]
	}

	return key, nil
}

// doRequest выполняет HTTP запрос и возвращает статус-код и тело ответа.
func (c *Client) doRequest(
	ctx context.Context, method, url string,
	headers map[string]string, body []byte,
) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return 0, nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("ошибка запроса: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, nil, fmt.Errorf("ошибка чтения ответа: %w", readErr)
	}

	return resp.StatusCode, respBody, nil
}

// CreateTransaction создает платеж в Platega и возвращает ссылку для оплаты и ID транзакции.
//
//nolint:cyclop,funlen
func (c *Client) CreateTransaction(
	ctx context.Context,
	amount float64,
	orderID string,
) (paymentURL, externalID string, err error) {
	defer c.logDuration("CreateTransaction", &err)()

	if amount <= 0 {
		err = errors.New("platega.CreateTransaction: amount должен быть > 0")

		return
	}

	merchantID, apiKey, returnURL, appErr := c.resolveCredentials()
	if appErr != nil {
		err = appErr

		return
	}

	jsonData, marshalErr := json.Marshal(CreateTransactionV2Request{
		PaymentDetails: PaymentDetails{Amount: amount, Currency: "RUB"},
		Description:    orderID,
		ReturnURL:      returnURL,
	})
	if marshalErr != nil {
		err = fmt.Errorf("platega.CreateTransaction: ошибка маршалинга: %w", marshalErr)

		return
	}

	idempotenceKey, keyErr := newIdempotenceKey(orderID)
	if keyErr != nil {
		err = fmt.Errorf("platega.CreateTransaction: %w", keyErr)

		return
	}

	url := strings.TrimRight(c.baseURL, "/") + "/v2/transaction/process"

	statusCode, respBody, reqErr := c.doRequest(ctx, http.MethodPost, url, map[string]string{
		"X-MerchantId":    merchantID,
		"X-Secret":        apiKey,
		"Content-Type":    "application/json",
		"Idempotence-Key": idempotenceKey,
	}, jsonData)
	if reqErr != nil {
		err = fmt.Errorf("platega.CreateTransaction: %w", reqErr)

		return
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		err = fmt.Errorf("platega.CreateTransaction: статус %d, ответ: %s",
			statusCode, string(respBody))

		return
	}

	var txResp CreateTransactionV2Response
	if unmarshalErr := json.Unmarshal(respBody, &txResp); unmarshalErr != nil {
		err = fmt.Errorf("platega.CreateTransaction: ошибка парсинга ответа: %w", unmarshalErr)

		return
	}

	if txResp.URL == "" || txResp.TransactionID == "" {
		err = errors.New("platega.CreateTransaction: пустой url или transactionId в ответе")

		return
	}

	paymentURL = txResp.URL
	externalID = txResp.TransactionID

	return
}

// CheckStatus проверяет статус транзакции.
func (c *Client) CheckStatus(
	ctx context.Context, transactionID string,
) (_ domain.PaymentStatus, err error) {
	defer c.logDuration("CheckStatus", &err)()

	info, getErr := c.GetTransactionInfo(ctx, transactionID)
	if getErr != nil {
		err = getErr

		return
	}

	switch info.GetStatus() {
	case "CONFIRMED":
		return domain.PaymentStatusSuccess, nil
	case "PENDING":
		return domain.PaymentStatusPending, nil
	case "CANCELED", "CHARGEBACKED":
		return domain.PaymentStatusFailed, nil
	default:
		return domain.PaymentStatusPending, nil
	}
}

// WaitForPayment блокирует выполнение до подтверждения/отмены платежа или таймаута.
// Первые 5 минут проверяет каждые 10 секунд, затем до 20 минут — каждые 60 секунд.
func (c *Client) WaitForPayment(
	ctx context.Context, transactionID string,
) (_ domain.PaymentStatus, err error) {
	defer c.logDuration("WaitForPayment", &err)()

	if strings.TrimSpace(transactionID) == "" {
		err = errors.New("platega.WaitForPayment: transactionID пустой")

		return
	}

	const (
		frequentInterval = 10 * time.Second
		sparseInterval   = 60 * time.Second
		frequentDuration = 5 * time.Minute
		totalDuration    = 20 * time.Minute
	)

	deadline := time.Now().Add(totalDuration)
	frequentDeadline := time.Now().Add(frequentDuration)

	for time.Now().Before(deadline) {
		status, checkErr := c.CheckStatus(ctx, transactionID)
		if checkErr != nil {
			c.logger.Error("WaitForPayment: ошибка проверки статуса",
				logger.Field{Key: "transactionID", Value: transactionID},
				logger.Field{Key: "err", Value: checkErr.Error()},
			)
		} else if status == domain.PaymentStatusSuccess || status == domain.PaymentStatusFailed {
			return status, nil
		}

		var interval time.Duration
		if time.Now().Before(frequentDeadline) {
			interval = frequentInterval
		} else {
			interval = sparseInterval
		}

		select {
		case <-ctx.Done():
			err = ctx.Err()

			return
		case <-time.After(interval):
		}
	}

	err = errors.New("platega.WaitForPayment: таймаут ожидания платежа")

	return
}

// GetTransactionInfo получает информацию о транзакции по её ID.
func (c *Client) GetTransactionInfo(
	ctx context.Context, transactionID string,
) (_ domain.TransactionInfo, err error) {
	defer c.logDuration("GetTransactionInfo", &err)()

	if strings.TrimSpace(transactionID) == "" {
		err = errors.New("platega.GetTransactionInfo: transactionID пустой")

		return
	}

	merchantID, apiKey, _, appErr := c.resolveCredentials()
	if appErr != nil {
		err = appErr

		return
	}

	url := fmt.Sprintf("%s/transaction/%s", strings.TrimRight(c.baseURL, "/"), transactionID)

	statusCode, body, reqErr := c.doRequest(ctx, http.MethodGet, url, map[string]string{
		"X-MerchantId": merchantID,
		"X-Secret":     apiKey,
	}, nil)
	if reqErr != nil {
		err = fmt.Errorf("platega.GetTransactionInfo: %w", reqErr)

		return
	}

	if statusCode != http.StatusOK {
		err = fmt.Errorf("platega.GetTransactionInfo: статус %d, ответ: %s",
			statusCode, string(body))

		return
	}

	var txInfo TransactionInfoResponse
	if unmarshalErr := json.Unmarshal(body, &txInfo); unmarshalErr != nil {
		err = fmt.Errorf("platega.GetTransactionInfo: ошибка парсинга JSON: %w", unmarshalErr)

		return
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

// resolveCredentials возвращает merchantID, apiKey, returnURL.
// Все значения уже проставлены в конструкторе или берутся из env в нём же.
func (c *Client) resolveCredentials() (merchantID, apiKey, returnURL string, err error) {
	merchantID = strings.TrimSpace(c.merchantID)
	apiKey = strings.TrimSpace(c.apiKey)
	returnURL = strings.TrimSpace(c.returnURL)

	if merchantID == "" {
		return "", "", "", errors.New("platega: не задан merchantID")
	}

	if apiKey == "" {
		return "", "", "", errors.New("platega: не задан apiKey")
	}

	if returnURL == "" {
		return "", "", "", errors.New("platega: не задан returnURL")
	}

	return merchantID, apiKey, returnURL, nil
}
