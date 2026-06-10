// Package youkassa реализует простой клиент для работы с платежной системой ЮKassa.
// Его задача: создать платеж и вернуть ссылку на оплату (confirmation_url), а также уметь проверить статус платежа.
package youkassa

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
	"strconv"
	"strings"
	"time"

	payment "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	"go.uber.org/zap"
)

// Client хранит настройки и зависимости для общения с API ЮKassa.
// Он реализует интерфейс billingSvc.PaymentGateway.
type Client struct {
	baseURL    string
	shopID     string
	secretKey  string
	returnURL  string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient создает клиент ЮKassa.
// Если какие-то параметры не переданы (пустые строки), клиент попытается взять их из переменных окружения.
func NewClient(shopID, secretKey, returnURL string, l *zap.Logger) *Client {
	if l == nil {
		l = zap.NewNop()
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
// orderID - ID заказа внутри вашей системы (пойдет в description и в ключ идемпотентности).
func (c *Client) CreateTransaction(
	ctx context.Context,
	amount float64,
	orderID string,
) (paymentURL, externalID string, err error) {
	defer c.logDuration("CreateTransaction")()

	// 1) Валидация данных
	if amount <= 0 {
		return "", "", errors.New("youkassa.CreateTransaction: amount должен быть > 0")
	}

	shopID, secretKey, returnURL, err := c.resolveCredentials()
	if err != nil {
		return "", "", err
	}

	// 2) Создание и подготовка запроса
	httpReq, err := c.newPaymentRequest(ctx, shopID, secretKey, returnURL, amount, orderID)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: %w", err)
	}

	// 3) Выполнение сетевого запроса
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf(
			"youkassa.CreateTransaction: ошибка выполнения HTTP запроса: %w",
			err,
		)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Error(
				"ошибка при закрытии тела ответа",
				zap.String("method", "CreateTransaction"),
				zap.String("err_msg", closeErr.Error()),
			)
		}
	}()

	// 4) Чтение и парсинг ответа (вынесено)
	confirmationURL, paymentID, err := parsePaymentResponse(httpResp)
	if err != nil {
		return "", "", fmt.Errorf("youkassa.CreateTransaction: %w", err)
	}

	return confirmationURL, paymentID, nil
}

// Вспомогательный метод для сборки и настройки HTTP-запроса.
func (c *Client) newPaymentRequest(
	ctx context.Context,
	shopID, secretKey, returnURL string,
	amount float64,
	orderID string,
) (*http.Request, error) {
	reqBody := createPaymentRequest{
		Amount: amountDTO{
			Value:    strconv.FormatFloat(amount, 'f', 2, 64),
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: confirmationDTO{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Description: "Заказ №" + orderID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга JSON: %w", err)
	}

	idempotenceKey, err := newIdempotenceKey(orderID)
	if err != nil {
		return nil, fmt.Errorf("не удалось сгенерировать Idempotence-Key: %w", err)
	}

	url := strings.TrimRight(c.baseURL, "/") + "/v3/payments"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.SetBasicAuth(shopID, secretKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", idempotenceKey)

	return httpReq, nil
}

// Вспомогательная функция для обработки ответа от ЮKassa.
func parsePaymentResponse(resp *http.Response) (string, string, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("ошибка чтения тела ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("статус %d, ответ: %s", resp.StatusCode, string(respBody))
	}

	var payment createPaymentResponse
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return "", "", fmt.Errorf("ошибка парсинга JSON ответа: %w", err)
	}

	confirmationURL := strings.TrimSpace(payment.Confirmation.ConfirmationURL)
	if confirmationURL == "" {
		return "", "", errors.New("в ответе нет confirmation_url")
	}

	return confirmationURL, payment.ID, nil
}

// CheckStatus проверяет статус платежа и маппит его в доменные статусы проекта.
func (c *Client) CheckStatus(
	ctx context.Context,
	transactionID string,
) (payment.PaymentStatus, error) {
	defer c.logDuration("CheckStatus")()

	info, err := c.GetTransactionInfo(ctx, transactionID)
	if err != nil {
		return "", err
	}

	// Статусы ЮKassa: pending, waiting_for_capture, succeeded, canceled.
	switch info.GetStatus() {
	case "succeeded":
		return payment.PaymentStatusSuccess, nil
	case "pending", "waiting_for_capture":
		return payment.PaymentStatusPending, nil
	case "canceled":
		return payment.PaymentStatusFailed, nil
	default:
		// На всякий случай: неизвестный статус считаем pending, чтобы не “ронять” оплату.
		return payment.PaymentStatusPending, nil
	}
}

// GetTransactionInfo получает объект платежа из ЮKassa по его ID.
func (c *Client) GetTransactionInfo(
	ctx context.Context,
	transactionID string,
) (billingSvc.TransactionInfo, error) {
	defer c.logDuration("GetTransactionInfo")()

	shopID, secretKey, _, err := c.resolveCredentials()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(transactionID) == "" {
		return nil, errors.New("youkassa.GetTransactionInfo: transactionID пустой")
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
		return nil, fmt.Errorf(
			"youkassa.GetTransactionInfo: ошибка выполнения HTTP запроса: %w",
			err,
		)
	}
	defer func() error {
		err := httpResp.Body.Close()
		if err != nil {
			return err
		}

		return nil
	}()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("youkassa.GetTransactionInfo: ошибка чтения тела ответа: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"youkassa.GetTransactionInfo: статус %d, ответ: %s",
			httpResp.StatusCode,
			string(body),
		)
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
		return "", "", "", errors.New("youkassa: не задан shopID (YOOKASSA_SHOP_ID)")
	}

	if secretKey == "" {
		return "", "", "", errors.New("youkassa: не задан secretKey (YOOKASSA_SECRET_KEY)")
	}

	if returnURL == "" {
		return "", "", "", errors.New("youkassa: не задан returnURL (YOOKASSA_RETURN_URL)")
	}

	return shopID, secretKey, returnURL, nil
}

// logDuration логирует длительность выполнения метода.
func (c *Client) logDuration(method string) func() {
	start := time.Now()

	return func() {
		c.logger.Info(
			"вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
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
// Он также реализует billingSvc.TransactionInfo, чтобы проект мог работать с платежами через общий интерфейс.
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

// // nopLogger — безопасный "пустой" логгер на случай, если клиент создают без логгера.
// type nopLogger struct{}
//
// func (nopLogger) Info(string, ...zap.Field)  {}
// func (nopLogger) Error(string, ...zap.Field) {}
// func (nopLogger) Sync() error                { return nil }
