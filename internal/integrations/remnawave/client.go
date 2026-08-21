// Package remnawave клиент для работы с RemnaWave API.
//
//nolint:funlen, cyclop // Вынужденная мера, чтобы не усложнять код и он был более читаемым.
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

	"go.uber.org/zap"
)

// RemnawaveClient определяет контракт для работы с RemnaWave API.
type RemnawaveClient interface {
	GetUUIDByUsername(ctx context.Context, username string) (string, error)
	GetAllUsers(ctx context.Context) ([]User, error)
	CreateUser(dto CreateUserDTO) error
	DeleteUser(ctx context.Context, username string) error
	ExtendClientSubscription(userUUID string, username string, days int) error
	GetUserInfo(uuid string) (GetUserInfoResponse, error)
	GetUserDevice(ctx context.Context, username string) ([]HWIDDevice, error)
	SetDevices(ctx context.Context, username string, devices *uint8) error
	BetterResetTraffic(ctx context.Context, username string) error
	DeleteDeviceHWID(ctx context.Context, username string) error
}

// RemnaConfig хранит важные *config данные из env.
type RemnaConfig struct {
	PanelURL           string
	SecretURLToken     string
	APIKey             string
	SquadUUID          string
	TrafficLimitGB     int64
	DefaultDeviceLimit int
}

// RemnaClient описывает то что нужно для работы remnawave.
type RemnaClient struct {
	cfg        RemnaConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewRemnaClient конструктор для создания клиента.
func NewRemnaClient(cfg RemnaConfig, l *zap.Logger) *RemnaClient {
	return &RemnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			// Хорошая практика: всегда задавать тайм-аут.
			Timeout: 10 * time.Second,
			// Указываем все поля, так как ругаются линтер.
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
		},
		logger: l,
	}
}

// EncryptURL метод, который шифрует URL.
func (c *RemnaClient) EncryptURL(url string) (string, error) {
	defer c.logDuration("EncryptURL")()

	data := &EncryptURLRequest{URL: url}

	jsonData, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(
			"failed to marshal json",
			zap.Error(err),
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	apiURL := "https://crypto.happ.su/api.php"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		c.logger.Error(
			"failed to make doRequest",
			zap.Error(err),
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		username := "заглушка" // заглушка для wrapErr
		c.wrapErr(err, "Do", username, url)
		c.logger.Error(
			"failed to get response",
			zap.Error(err),
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer c.closeBody(response)

	if response.StatusCode != http.StatusOK {
		body, err := c.readBody(response)
		if err != nil {
			return "", err
		}

		c.logger.Error(
			"плохой status code",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", body),
		)

		return "", fmt.Errorf("%w: %d", ErrBadStatusCode, response.StatusCode)
	}

	body, err := c.readBody(response)
	if err != nil {
		return "", err
	}

	var encResponse EncryptURLResponse

	err = c.parseJSON(body, &encResponse)
	if err != nil {
		return "", err
	}

	if encResponse.EncryptedLink == "" {
		c.logger.Error(
			"ошибка получения зашифрванной подписки",
			zap.String("response_body", body),
		)

		return "", ErrEmptyURL
	}

	return encResponse.EncryptedLink, nil
}

// doRequest делает запросы к remnawave с стандартными заголовками.
// При временных сетевых ошибках (таймаут, connection refused) повторяет запрос.
func (c *RemnaClient) doRequest(
	ctx context.Context,
	method, url string,
	body any,
) (*http.Response, error) {
	maxRetries := 3
	backoff := 1 * time.Second
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var bodyReader io.Reader

		if body != nil {
			jsonData, err := json.Marshal(body)
			if err != nil {
				c.logger.Error(
					"ошибка парсинга json",
					zap.Error(err),
					zap.String("method", method),
					zap.String("url", url),
				)

				return nil, fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
			}

			bodyReader = bytes.NewBuffer(jsonData)
		} else {
			bodyReader = http.NoBody
		}

		request, err := http.NewRequestWithContext(
			ctx,
			method,
			url,
			bodyReader,
		)
		if err != nil {
			c.logger.Error(
				"ошибка создания запроса",
				zap.Error(err),
				zap.String("method", method),
				zap.String("url", url),
			)

			return nil, fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
		}

		request.Header.Add("Content-Type", "application/json")
		request.Header.Add("Authorization", "Bearer "+c.cfg.APIKey)

		response, err := c.httpClient.Do(request)
		if err == nil {
			return response, nil // успешно
		}

		lastErr = err
		c.logger.Warn(
			"сетевая ошибка, повторяем запрос",
			zap.Error(err),
			zap.String("method", method),
			zap.String("url", url),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
		)

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2 // 1s → 2s → 4s
		}
	}

	c.logger.Error(
		"failed to execute doRequest after retries",
		zap.Error(lastErr),
		zap.String("method", method),
		zap.String("url", url),
	)

	return nil, fmt.Errorf("%w: %w", ErrFailedToDoRequest, lastErr)
}

// readBody читает тело ответа с логированием ошибок.
func (c *RemnaClient) readBody(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error(
			"failed to read response body",
			zap.String("error", err.Error()),
			zap.Int("status_code", resp.StatusCode),
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToReadBody, err)
	}

	bodyStr := string(body)

	return bodyStr, nil
}

// closeBody безопасно закрывает тело ответа с логированием.
func (c *RemnaClient) closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		c.logger.Error(
			"failed to close response body",
			zap.String("error", err.Error()),
		)
	}
}

// parseJSON парсит JSON с логированием при ошибке.
func (c *RemnaClient) parseJSON(data string, target any) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		c.logger.Error(
			"failed to unmarshal JSON",
			zap.String("error", err.Error()),
			zap.String("response_body", data),
		)

		return fmt.Errorf("%w: %w", ErrUnmarshal, err)
	}

	return nil
}

// doRequestAndParse - выполняет HTTP запрос, проверяет статус код, читает и парсит ответ
// Применяет DRY - используется во всех методах клиентской части.
func (c *RemnaClient) doRequestAndParse(
	ctx context.Context,
	method, url string,
	body any,
	response any,
	logField string, // для логов (username или uuid)
	// userData GetUUIDByUsernameResponse,
) error {
	// Выполняем базовый HTTP запрос
	resp, err := c.doRequest(ctx, method, url, body)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	// Обработка статус кодов ответа
	switch resp.StatusCode {
	case http.StatusOK:
		// Все ок, продолжаем парсить
	case http.StatusNotFound:
		return ErrNotFound
	default:
		// Для ошибок читаем тело ответа для логирования
		respBody, readErr := c.readBody(resp)
		if readErr != nil {
			c.logger.Error("failed to read error response")
			respBody = ""
		}

		c.logger.Error(
			"ошибка HTTP ответа",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", respBody),
			zap.String("field", logField),
		)

		return ErrInternalServerError
	}

	// Читаем тело ответа при успешном запросе
	respBody, err := c.readBody(resp)
	if err != nil {
		return err
	}

	// Парсим JSON в переданную структуру
	if err := c.parseJSON(respBody, response); err != nil {
		return err
	}

	return nil
}

// handleUpdate обработка PATCH/PUT запросов.
func (c *RemnaClient) handleUpdate(response *http.Response, bodyStr string) (string, error) {
	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return bodyStr, nil
	case http.StatusBadRequest:
		c.logger.Error(
			"bad doRequest while updating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, ErrBadRequest
	case http.StatusNotFound:
		c.logger.Error(
			"not found while updating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, ErrNotFound
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error while updating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, ErrInternalServerError
	default:
		c.logger.Error(
			"unexpected status code while updating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, fmt.Errorf("%w: %d", ErrUndefined, response.StatusCode)
	}
}

// handleCreate - обработка POST запросов, создание.
func (c *RemnaClient) handleCreate(response *http.Response, bodyStr string) (string, error) {
	switch response.StatusCode {
	case http.StatusCreated, http.StatusOK:
		return bodyStr, nil
	case http.StatusBadRequest:
		// Проверяем, является ли ошибка "User username already exists"
		if strings.Contains(bodyStr, "User username already exists") {
			c.logger.Info("Пользователь уже существует, пропускаем создание")

			return bodyStr, nil // Возвращаем успех, если пользователь уже существует
		}

		c.logger.Error(
			"bad doRequest while creating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, ErrBadRequestCreate
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error while creating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, ErrInternalServerError
	default:
		c.logger.Error(
			"unexpected status code while creating",
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return bodyStr, fmt.Errorf("%w: %d", ErrUndefined, response.StatusCode)
	}
}

// handleDelete обрабатывает ответ DELETE запроса.
func (c *RemnaClient) handleDelete(response *http.Response, bodyStr, username, uuid string) error {
	switch response.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		c.logger.Info(
			"user successfully deleted",
			zap.String("username", username),
			zap.String("UUID", uuid),
			zap.Int("status_code", response.StatusCode),
		)

		return nil

	case http.StatusNotFound:
		c.logger.Warn(
			"user not found during deletion",
			zap.String("username", username),
			zap.String("UUID", uuid),
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return ErrNotFound

	case http.StatusUnauthorized:
		c.logger.Error(
			"authorization failed during user deletion",
			zap.String("username", username),
			zap.String("UUID", uuid),
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return errors.New("authorization error - check authentication token")

	default:
		c.logger.Error(
			"unexpected status code during user deletion",
			zap.String("username", username),
			zap.String("UUID", uuid),
			zap.Int("status_code", response.StatusCode),
			zap.String("response_body", bodyStr),
		)

		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
}

// wrapErr убирает дублирование обработки ошибок.
func (c *RemnaClient) wrapErr(err error, msg, username string, url ...string) error {
	if err == nil {
		return nil
	}

	fields := []zap.Field{
		zap.String("username", username),
		zap.Error(err),
	}

	if len(url) > 0 && url[0] != "" {
		fields = append(fields, zap.String("url", url[0]))
	}

	c.logger.Error(msg, fields...)

	return fmt.Errorf("%s: %w", msg, err)
}

// changeUserState изменяет состояние пользователя в панели Remnawave.
func (c *RemnaClient) changeUserState(userUUID, action string) error {
	url := c.actionURL(userUUID, action)

	resp, err := c.doRequest(context.Background(), http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	// Читаем тело ответа для логирования
	body, _ := c.readBody(resp)

	_, err = c.handleUpdate(resp, body)
	if err != nil {
		if errors.Is(err, ErrBadRequest) {
			return ErrBadRequestUUID
		}

		return err
	}
	c.logger.Info(
		"метод включения/выключения клиента успешно отработал",
		zap.String("userUUID", userUUID),
	)

	return nil
}

// actionURL отдаем методам строку, чтобы избежать дублирование кода в методах.
func (c *RemnaClient) actionURL(userUUID string, action string) string {
	return fmt.Sprintf(
		"%s/api/users/%s/actions/%s?%s",
		c.cfg.PanelURL,
		userUUID,
		action,
		c.cfg.SecretURLToken,
	)
}

// logDuration логирует время выполнения метода.
func (c *RemnaClient) logDuration(method string) func() {
	start := time.Now()

	return func() {
		c.logger.Info(
			"вызов метода завершен",
			zap.String("method", method),
			zap.Duration("duration", time.Since(start)),
		)
	}
}
