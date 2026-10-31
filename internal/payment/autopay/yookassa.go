package autopay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const yookassaBaseURL = "https://api.yookassa.ru"

type yooClient struct {
	shopID     string
	secretKey  string
	returnURL  string
	baseURL    string
	httpClient *http.Client
}

func newYooClient(shopID, secretKey, returnURL string) *yooClient {
	return &yooClient{
		shopID:     shopID,
		secretKey:  secretKey,
		returnURL:  returnURL,
		baseURL:    yookassaBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type yooAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type yooConfirmation struct {
	Type      string `json:"type"`
	ReturnURL string `json:"return_url"`
}

type yooCreatePayment struct {
	Amount            yooAmount         `json:"amount"`
	Capture           bool              `json:"capture"`
	Confirmation      yooConfirmation   `json:"confirmation"`
	Description       string            `json:"description"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	SavePaymentMethod bool              `json:"save_payment_method"`
}

type yooCharge struct {
	Amount          yooAmount `json:"amount"`
	Capture         bool      `json:"capture"`
	PaymentMethodID string    `json:"payment_method_id"`
	Description     string    `json:"description"`
}

type yooPayment struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
	Amount        yooAmount         `json:"amount"`
	Metadata      map[string]string `json:"metadata"`
	PaymentMethod struct {
		ID    string `json:"id"`
		Saved bool   `json:"saved"`
	} `json:"payment_method"`
}

func (c *yooClient) CreateBindingPayment(
	ctx context.Context,
	userID string,
	amountKopecks int,
) (string, string, error) {
	body := yooCreatePayment{
		Amount: yooAmount{
			Value:    rubles(amountKopecks),
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: yooConfirmation{
			Type:      "redirect",
			ReturnURL: c.returnURL,
		},
		Description:       "Оплата подписки",
		Metadata:          map[string]string{"userID": userID},
		SavePaymentMethod: true,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	resp, err := c.do(ctx, http.MethodPost, "/v3/payments", raw, uuid.NewString())
	if err != nil {
		return "", "", err
	}

	var out yooPayment
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", "", err
	}

	if out.ID == "" || out.Confirmation.ConfirmationURL == "" {
		return "", "", fmt.Errorf("yookassa: платёж без id или ссылки: %s", resp)
	}

	return out.ID, out.Confirmation.ConfirmationURL, nil
}

func (c *yooClient) Charge(
	ctx context.Context,
	methodID string,
	amountKopecks int,
	idempotenceKey string,
) (string, error) {
	if methodID == "" {
		return "", errors.New("yookassa: пустой payment_method_id")
	}
	if idempotenceKey == "" {
		return "", errors.New("yookassa: пустой Idempotence-Key")
	}

	raw, err := json.Marshal(yooCharge{
		Amount: yooAmount{
			Value:    rubles(amountKopecks),
			Currency: "RUB",
		},
		Capture:         true,
		PaymentMethodID: methodID,
		Description:     "Продление подписки",
	})
	if err != nil {
		return "", err
	}

	resp, err := c.do(ctx, http.MethodPost, "/v3/payments", raw, idempotenceKey)
	if err != nil {
		return "", err
	}

	var out yooPayment
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", err
	}

	if out.ID == "" {
		return "", fmt.Errorf("yookassa: автоплатёж без id: %s", resp)
	}

	return out.ID, nil
}

func (c *yooClient) Status(ctx context.Context, paymentID string) (string, error) {
	payment, err := c.Payment(ctx, paymentID)
	if err != nil {
		return "", err
	}

	return payment.Status, nil
}

func (c *yooClient) Payment(
	ctx context.Context,
	paymentID string,
) (yooPayment, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v3/payments/"+paymentID, nil, "")
	if err != nil {
		return yooPayment{}, err
	}

	var out yooPayment
	if err := json.Unmarshal(resp, &out); err != nil {
		return yooPayment{}, err
	}

	return out, nil
}

func (c *yooClient) SavedMethod(
	ctx context.Context,
	paymentID string,
) (string, bool, error) {
	payment, err := c.Payment(ctx, paymentID)
	if err != nil {
		return "", false, err
	}

	if !payment.PaymentMethod.Saved || payment.PaymentMethod.ID == "" {
		return "", false, nil
	}

	return payment.PaymentMethod.ID, true, nil
}

func (c *yooClient) do(
	ctx context.Context,
	method,
	path string,
	body []byte,
	idempotenceKey string,
) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		reader,
	)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.shopID, c.secretKey)
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotence-Key", idempotenceKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("yookassa status %d: %s", resp.StatusCode, raw)
	}

	return raw, nil
}

func rubles(kopecks int) string {
	return fmt.Sprintf("%d.%02d", kopecks/100, kopecks%100)
}
