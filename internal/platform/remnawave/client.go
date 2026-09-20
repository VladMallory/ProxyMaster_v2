package platformremnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotFound = errors.New("страница не найдена")

type Client struct {
	baseURL,
	token string
	http *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func Do[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var result T
	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return result, err
		}

		reader = bytes.NewReader(data)
	}

	fullURL := strings.TrimRight(c.baseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return result, err
	}
	defer closer(resp.Body, &err)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return result, ErrNotFound
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("ошибка запроса: %d: %s", resp.StatusCode, raw)
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}

	return result, nil
}

func closer(closer io.Closer, err *error) {
	if cerr := closer.Close(); cerr != nil {
		*err = errors.Join(*err, cerr)
	}
}
