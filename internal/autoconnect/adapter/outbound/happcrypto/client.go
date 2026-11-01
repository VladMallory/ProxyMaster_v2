package happcrypto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/closer"
)

// maxResponseSize ограничивает чтение ответа: чужой сервис не должен
// заставить нас держать в памяти мегабайты.
const maxResponseSize = 8 << 10

// Client шифрует ссылки подписки через официальный API Happ.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type request struct {
	URL string `json:"url"`
}

type encryptRespons struct {
	EncryptedLink string `json:"encrypted_link"`
}

// Encrypt возвращает happ://crypt5/<payload> для ссылки подписки.
func (c *Client) Encrypt(ctx context.Context, subscriptionURL string) (string, error) {
	resp, err := Do[encryptRespons](
		ctx,
		c,
		request{URL: subscriptionURL},
	)
	if err != nil {
		return "", err
	}

	link := strings.TrimSpace(resp.EncryptedLink)
	if !strings.HasPrefix(link, "happ://crypt5/") {
		return "", fmt.Errorf("happ crypto: unexpected link %q", link)
	}

	return link, nil
}

func Do[T any](ctx context.Context, c *Client, body any) (T, error) {
	var result T

	data, err := json.Marshal(body)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		bytes.NewReader(data),
	)
	if err != nil {
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer func() { closer.HelpClose(resp.Body, &err) }()

	if resp.StatusCode != http.StatusOK {
		return result, err
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}

	return result, nil
}
