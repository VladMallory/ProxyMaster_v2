package remnawave

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

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

type RemnawaveClient struct {
	baseURL string
	token   string
	apiKey  string
	client  *http.Client
}

func NewRemnawaveClient(baseURL, token, apiKey string) *RemnawaveClient {
	return &RemnawaveClient{
		baseURL: baseURL,
		token:   token,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func doRequest[T any](
	ctx context.Context,
	client *http.Client,
	baseURL string,
	token string,
	method string,
	path string,
	body any,
) (T, error) {
	var result T

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return result, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	fullURL := strings.TrimRight(baseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("do request: %w", err)
	}
	defer closer(resp.Body, &err)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return result, subdomain.ErrNoFindUser
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("request failed %d: %s", resp.StatusCode, raw)
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return result, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}

func closer(closer io.Closer, err *error) {
	if cerr := closer.Close(); cerr != nil {
		_ = errors.Join(*err, cerr)
	}
}
