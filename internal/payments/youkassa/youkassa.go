// Package youkassa реализует простой клиент для работы с платежной системой ЮKassa.
// Его задача: создать платеж и вернуть ссылку на оплату (confirmation_url), а также уметь проверить статус платежа.
package youkassa

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/pkg/logger"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client хранит настройки и зависимости для общения с API ЮKassa.
// Он реализует интерфейс domain.PaymentGateway.
type Client struct {
	baseURL    string
	shopID     string
	secretKey  string
	returnURL  string
	httpClient *http.Client
	logger     logger.Logger
}

// NewClient создает клиент ЮKassa.
// Если какие-то параметры не переданы (пустые строки), клиент попытается взять их из переменных окружения.
func NewClient(shopID, secretKey, returnURL string, l logger.Logger) *Client {
	// В этом проекте логгер везде внедряется зависимостью.
	// Но чтобы клиент было проще использовать в тестах/черновиках, делаем безопасный fallback.
	if l == nil {
		l = nopLogger{}
	}

	c := &Client{
		baseURL:   "https://api.yookassa.ru",
		shopID:    shopID,
		secretKey: secretKey,
		returnURL: returnURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: l,
	}

	// Если в коде baseURL когда-то захотите менять (например, на песочницу) — можно задать через env.
	if v := strings.TrimSpace(os.Getenv("YOOKASSA_BASE_URL")); v != "" {
		c.baseURL = v
	}

	return c
}

// CreateTransaction создает платеж в ЮKassa и возвращает ссылку для оплаты и внешний ID платежа.
//
// Amount  - сумма в рублях (например 100.00)
// orderID - ID заказа внутри вашей системы (пойдет в description и в ключ идемпотентности)
func (c *Client) CreateTransaction(ctx context.Context, amount float64, orderID string) (paymentURL, externalID string, err error) {
	defer c.logDuration("CreateTransaction")()

	// 1) Проверяем входные данные
	if amount <= 0 {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: amount должен быть > 0")
	}

	// 2) Достаем креды/return_url (из клиента или из env)
	shopID, secretKey, returnURL, err := c.resolveCredentials()
	if err != nil {
		return "", "", err
	}

	// 3) Собираем запрос как в документации ЮKassa
	// Важно: value — строка с двумя знаками после запятой.
	reqBody := createPaymentRequest{
		Amount: amountDTO{
			Value:    fmt.Sprintf("%.2f", amount),
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: confirmationDTO{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Description: fmt.Sprintf("Заказ №%s", orderID),
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: ошибка маршалинга JSON: %w", err)
	}

	// 4) ЮKassa требует Idempotence-Key — чтобы повторный запрос не создал дубликат платежа.
	idempotenceKey, err := newIdempotenceKey(orderID)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: не удалось сгенерировать Idempotence-Key: %w", err)
	}

	// 5) Делаем HTTP запрос
	url := strings.TrimRight(c.baseURL, "/") + "/v3/payments"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: ошибка создания HTTP запроса: %w", err)
	}

	// BasicAuth: shopID:secretKey
	httpReq.SetBasicAuth(shopID, secretKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", idempotenceKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: ошибка выполнения HTTP запроса: %w", err)
	}

	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Error(
				"ошибка при закрытии тела ответа",
				logger.Field{Key: "method", Value: "CreateTransaction"},
				logger.Field{Key: "err_msg", Value: closeErr.Error()},
			)
		}
	}()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: ошибка чтения тела ответа: %w", err)
	}

	// В документации часто встречается 200, но некоторые API возвращают 201 при создании.
	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: статус %d, ответ: %s", httpResp.StatusCode, string(respBody))
	}

	var payment createPaymentResponse
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: ошибка парсинга JSON ответа: %w", err)
	}

	// 6) Для сценария redirect нам нужна ссылка payment.confirmation.confirmation_url
	confirmationURL := strings.TrimSpace(payment.Confirmation.ConfirmationURL)
	if confirmationURL == "" {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: в ответе нет confirmation_url")
	}

	return confirmationURL, payment.ID, nil
}

// CheckStatus проверяет статус платежа и маппит его в доменные статусы проекта.
func (c *Client) CheckStatus(ctx context.Context, transactionID string) (domain.PaymentStatus, error) {
	defer c.logDuration("CheckStatus")()

	info, err := c.GetTransactionInfo(ctx, transactionID)
	if err != nil {
		return "", err
	}

	// Статусы ЮKassa: pending, waiting_for_capture, succeeded, canceled.
	switch info.GetStatus() {
	case "succeeded":
		return domain.PaymentStatusSuccess, nil
	case "pending", "waiting_for_capture":
		return domain.PaymentStatusPending, nil
	case "canceled":
		return domain.PaymentStatusFailed, nil
	default:
		// На всякий случай: неизвестный статус считаем pending, чтобы не “ронять” оплату.
		return domain.PaymentStatusPending, nil
	}
}

// GetTransactionInfo получает объект платежа из ЮKassa по его ID.
func (c *Client) GetTransactionInfo(ctx context.Context, transactionID string) (domain.TransactionInfo, error) {
	defer c.logDuration("GetTransactionInfo")()

	shopID, secretKey, _, err := c.resolveCredentials()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(transactionID) == "" {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: transactionID пустой")
	}

	url := strings.TrimRight(c.baseURL, "/") + "/v3/payments/" + transactionID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: ошибка создания HTTP запроса: %w", err)
	}

	httpReq.SetBasicAuth(shopID, secretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: ошибка выполнения HTTP запроса: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(httpResp.Body)

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: ошибка чтения тела ответа: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: статус %d, ответ: %s", httpResp.StatusCode, string(body))
	}

	var payment createPaymentResponse
	if err := json.Unmarshal(body, &payment); err != nil {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: ошибка парсинга JSON: %w", err)
	}

	return &payment, nil
}

// resolveCredentials возвращает shopID, secretKey и returnURL.
// Сначала берем значения из Client, если они пустые — пробуем env.
func (c *Client) resolveCredentials() (shopID, secretKey, returnURL string, err error) {
	shopID = strings.TrimSpace(c.shopID)
	if shopID == "" {
		shopID = strings.TrimSpace(os.Getenv("YOOKASSA_SHOP_ID"))
	}

	secretKey = strings.TrimSpace(c.secretKey)
	if secretKey == "" {
		secretKey = strings.TrimSpace(os.Getenv("YOOKASSA_SECRET_KEY"))
	}

	returnURL = strings.TrimSpace(c.returnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(os.Getenv("YOOKASSA_RETURN_URL"))
	}

	if shopID == "" {
		return "", "", "", fmt.Errorf("youkassa: не задан shopID (YOOKASSA_SHOP_ID)")
	}

	if secretKey == "" {
		return "", "", "", fmt.Errorf("youkassa: не задан secretKey (YOOKASSA_SECRET_KEY)")
	}

	if returnURL == "" {
		return "", "", "", fmt.Errorf("youkassa: не задан returnURL (YOOKASSA_RETURN_URL)")
	}

	return shopID, secretKey, returnURL, nil
}

// logDuration логирует длительность выполнения метода.
func (c *Client) logDuration(method string) func() {
	start := time.Now()

	return func() {
		c.logger.Info(
			"вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

// newIdempotenceKey генерирует Idempotence-Key для ЮKassa.
// Идея простая: берем случайные байты и (если есть) добавляем orderID как префикс.
func newIdempotenceKey(orderID string) (string, error) {
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}

	rndHex := hex.EncodeToString(rnd)

	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		// rndHex длиной 32 символа — заведомо укладывается в лимит
		return rndHex, nil
	}

	// Ограничиваем длину orderID и убираем пробелы, чтобы ключ был стабильнее
	orderID = strings.ReplaceAll(orderID, " ", "_")
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	key := orderID + "-" + rndHex
	// На всякий случай обрезаем до 64 символов, чтобы удовлетворять лимиту ЮKassa
	if len(key) > 64 {
		key = key[:64]
	}

	return key, nil
}

// amountDTO описывает сумму платежа в формате ЮKassa.
type amountDTO struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// confirmationDTO описывает блок confirmation в ЮKassa.
type confirmationDTO struct {
	Type            string `json:"type"`
	ReturnURL       string `json:"return_url,omitempty"`
	ConfirmationURL string `json:"confirmation_url,omitempty"`
}

// createPaymentRequest описывает запрос создания платежа в ЮKassa.
type createPaymentRequest struct {
	Amount       amountDTO       `json:"amount"`
	Capture      bool            `json:"capture"`
	Confirmation confirmationDTO `json:"confirmation"`
	Description  string          `json:"description"`
}

// createPaymentResponse описывает ответ ЮKassa на создание/получение платежа.
// Он также реализует domain.TransactionInfo, чтобы проект мог работать с платежами через общий интерфейс.
type createPaymentResponse struct {
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	Paid         bool            `json:"paid"`
	Amount       amountDTO       `json:"amount"`
	Confirmation confirmationDTO `json:"confirmation"`
	CreatedAt    string          `json:"created_at"`
	Description  string          `json:"description"`
	Test         bool            `json:"test"`
}

// GetID возвращает ID платежа в ЮKassa.
func (p *createPaymentResponse) GetID() string {
	return p.ID
}

// GetAmount возвращает сумму платежа в виде float64.
func (p *createPaymentResponse) GetAmount() float64 {
	// ЮKassa хранит amount.value как строку.
	v := strings.TrimSpace(p.Amount.Value)
	if v == "" {
		return 0
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}

	return f
}

// GetStatus возвращает статус платежа из ЮKassa.
func (p *createPaymentResponse) GetStatus() string {
	return p.Status
}

// GetRawResponse возвращает исходный объект ответа, чтобы при необходимости можно было достать дополнительные поля.
func (p *createPaymentResponse) GetRawResponse() any {
	return p
}

// nopLogger — безопасный "пустой" логгер на случай, если клиент создают без логгера.
type nopLogger struct{}

// Debug ничего не делает.
func (nopLogger) Debug(string, ...logger.Field) {}

// Info ничего не делает.
func (nopLogger) Info(string, ...logger.Field) {}

// Warn ничего не делает.
func (nopLogger) Warn(string, ...logger.Field) {}

// Error ничего не делает.
func (nopLogger) Error(string, ...logger.Field) {}

// With возвращает тот же логгер.
func (nopLogger) With(...logger.Field) logger.Logger { return nopLogger{} }

// Named возвращает тот же логгер.
func (nopLogger) Named(string) logger.Logger { return nopLogger{} }

// Sync ничего не делает.
func (nopLogger) Sync() error { return nil }
