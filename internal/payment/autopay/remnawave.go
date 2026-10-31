package autopay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var errUserNotFound = errors.New("remnawave: пользователь не найден")

type remnaClient struct {
	baseURL    string
	apiKey     string
	token      string
	httpClient *http.Client
}

func newRemnaClient(baseURL, apiKey, token string) *remnaClient {
	return &remnaClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type panelUser struct {
	Response struct {
		ID       int    `json:"id"`
		ExpireAt string `json:"expireAt"`
	} `json:"response"`
}

func (c *remnaClient) ExpireAt(
	ctx context.Context,
	username string,
) (time.Time, error) {
	user, err := c.user(ctx, username)
	if err != nil {
		return time.Time{}, err
	}

	expireAt, err := time.Parse(time.RFC3339Nano, user.Response.ExpireAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("remnawave expireAt: %w", err)
	}

	return expireAt, nil
}

func (c *remnaClient) ExtendMonths(
	ctx context.Context,
	username string,
	months int,
) error {
	if months <= 0 {
		return errors.New("remnawave: months должен быть больше нуля")
	}

	user, err := c.user(ctx, username)
	if err != nil {
		return err
	}

	expireAt, err := time.Parse(time.RFC3339Nano, user.Response.ExpireAt)
	if err != nil {
		return fmt.Errorf("remnawave expireAt: %w", err)
	}

	base := expireAt
	if base.Before(time.Now()) {
		base = time.Now()
	}

	body, err := json.Marshal(map[string]any{
		"id":       user.Response.ID,
		"expireAt": base.AddDate(0, months, 0).UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	path := "/api/users/?" + c.apiKey

	resp, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}

	_ = resp

	return nil
}

func (c *remnaClient) user(
	ctx context.Context,
	username string,
) (panelUser, error) {
	path := "/api/users/by-username/" + url.PathEscape(username) + "?" + c.apiKey

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return panelUser{}, err
	}

	var out panelUser
	if err := json.Unmarshal(resp, &out); err != nil {
		return panelUser{}, err
	}

	return out, nil
}

func (c *remnaClient) do(
	ctx context.Context,
	method,
	path string,
	body []byte,
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

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

	if resp.StatusCode == http.StatusNotFound {
		return nil, errUserNotFound
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("remnawave status %d: %s", resp.StatusCode, raw)
	}

	return raw, nil
}
