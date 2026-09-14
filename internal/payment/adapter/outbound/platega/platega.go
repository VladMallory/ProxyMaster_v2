package platega

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	paymentdomain "github.com/VladMallory/ProxyMaster_v2/internal/payment/domain"
	"github.com/google/uuid"
)

type Client struct {
	baseURL    string
	merchantID string
	secret     string
	httpClient *http.Client
}

func NewClient(baseURL, merchantID, secret string) *Client {
	return &Client{
		baseURL:    baseURL,
		merchantID: merchantID,
		secret:     secret,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type createReq struct {
	ID             string         `json:"id"`
	PaymentMethod  int            `json:"paymentMethod"` // 11=карты, 2=СБП QR
	PaymentDetails paymentDetails `json:"paymentDetails"`
	Description    string         `json:"description,omitempty"`
	Return         string         `json:"return,omitempty"`
	FailedURL      string         `json:"failedUrl,omitempty"`
	Payload        string         `json:"payload,omitempty"` // userID для вебхука
}

type paymentDetails struct {
	Amount   float64 `json:"amount"`   // platega ждет float в рублях
	Currency string  `json:"currency"` // RUB
}

type createResp struct {
	TransactionID string `json:"transactionId"`
	Redirect      string `json:"redirect"` // ссылка на оплату
	Status        string `json:"status"`
}

func (c *Client) CreateInvoice(
	ctx context.Context,
	userID string,
	amount int,
) (string, string, error) {
	amountRub := float64(amount) / 100

	reqBody := createReq{
		ID:            uuid.NewString(),
		PaymentMethod: 2,
		PaymentDetails: paymentDetails{
			Amount:   amountRub,
			Currency: "RUB",
		},
		Description: "Оплата подписки",
		Return:      "https://google.com",
		FailedURL:   "https://google.com",
		Payload:     userID, // вернется в вебхуке чтобы найти юзера
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/transaction/process",
		bytes.NewReader(raw),
	)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Merchantid", c.merchantID)
	req.Header.Set("X-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("platega do: %w", err)
	}
	defer closerHelper(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("platega status %d", resp.StatusCode)
	}

	var out createResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode platega resp: %w", err)
	}

	return out.TransactionID, out.Redirect, nil
}

func (c *Client) CheckStatus(ctx context.Context, transactionID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/transaction/"+transactionID, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("X-Merchantid", c.merchantID)
	req.Header.Set("X-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer closerHelper(resp.Body, &err)

	var out paymentdomain.Out

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if err != nil {
			return false, err
		}
	}

	return out.Status == "CONFIRMED", nil
}

func closerHelper(closer io.Closer, err *error) {
	if cerr := closer.Close(); cerr != nil {
		*err = errors.Join(*err, cerr)
	}
}
