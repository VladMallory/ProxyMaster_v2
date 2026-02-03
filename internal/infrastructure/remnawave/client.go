// Package remnawave клиент для работы с RemnaWave API.
//
//nolint:funlen, cyclop // Вынужденная мера, чтобы не усложнять код и он был более читаемым.
package remnawave

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RemnaClient описывает то что нужно для работы remnawave.
type RemnaClient struct {
	cfg        *config.Config
	httpClient *http.Client
	// Храним логгер здесь для обращения к нему
	logger logger.Logger
}

// NewRemnaClient конструктор для создания клиента.
func NewRemnaClient(cfg *config.Config, l logger.Logger) *RemnaClient {
	l.Info("Создан экземпляр remnawave")

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

func (c *RemnaClient) AddInternalSquad(
	ctx context.Context,
	username string,
	squadTitles []string,
) error {
	c.logDuration("AddInternalSquad")()
	url := fmt.Sprintf("%s/api/users/?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	data := &models.UpdateUserRequest{
		Username:             &username,
		ActiveInternalSquads: squadTitles,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(
			"failed to marshal json",
			logger.Field{Key: "err_msg", Value: err},
		)

		return ErrFailedToMarshal
	}

	request, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	if err != nil {
		c.logger.Error(
			"failed to make doRequest",
			logger.Field{Key: "err_msg", Value: err},
		)

		return ErrFailedToMakeRequest
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to do doRequest",
			logger.Field{Key: "err_msg", Value: err},
		)

		return ErrFailedToDoRequest
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			c.logger.Error(
				"failed to close the body",
				logger.Field{Key: "err_msg", Value: err},
			)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error("failed to read body")

		return ErrFailedToMakeResponse
	}

	switch response.StatusCode {
	case http.StatusOK:
		c.logger.Info(
			"added internal squad successfully",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "squads added", Value: squadTitles},
		)

		return nil

	case http.StatusBadRequest:
		c.logger.Error(
			"failed to add internal squad",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return ErrBadRequest

	case http.StatusInternalServerError:
		c.logger.Error(
			"failed to add internal squad",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return ErrInternalServerError
	}

	return ErrUndefined
}

// EncryptURL метод, который шифрует URL.
func (c *RemnaClient) EncryptURL(url string) (string, error) {
	defer c.logDuration("EncryptURL")()

	data := &models.EncryptURLRequest{URL: url}

	jsonData, err := json.Marshal(data)
	if err != nil {
		c.logger.Error(
			"failed to marshal json",
			logger.Field{Key: "err_msg", Value: err},
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
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			c.logger.Error(
				"не удалось закрыть тело ответа",
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}()

	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			c.logger.Error(
				"failed to read error response body",
				logger.Field{Key: "err_msg", Value: readErr},
				logger.Field{Key: "status_code", Value: response.StatusCode},
			)

			return "", fmt.Errorf("%w: %d", ErrBadStatusCode, response.StatusCode)
		}

		c.logger.Error(
			"bad status code",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", fmt.Errorf("%w: %d", ErrBadStatusCode, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error(
			"failed to read response body",
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToReadBody, err)
	}

	var encResponse models.EncryptURLResponse
	if err := json.Unmarshal(body, &encResponse); err != nil {
		c.logger.Error(
			"failed to unmarshal json",
			logger.Field{Key: "err_msg", Value: err},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToUnmarshal, err)
	}

	if encResponse.EncryptedLink == "" {
		c.logger.Error(
			"received empty encrypted link",
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", ErrEmptyURL
	}

	return encResponse.EncryptedLink, nil
}

// doRequest делает запросы к remnawave с стандартными заголовками.
func (c *RemnaClient) doRequest(
	ctx context.Context,
	method, url string,
	body interface{},
) (*http.Response, error) {
	var bodyReader io.Reader

	// Сериализация тела запроса (если есть)
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			c.logger.Error(
				"ошибка парсинга json",
				logger.Field{Key: "err_msg", Value: err},
				logger.Field{Key: "method", Value: method},
				logger.Field{Key: "url", Value: url},
			)

			return nil, fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
		}

		bodyReader = bytes.NewBuffer(jsonData)
	} else {
		bodyReader = http.NoBody
	}

	// Создание HTTP запроса
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		url,
		bodyReader,
	)
	if err != nil {
		c.logger.Error(
			"ошибка создания запроса",
			logger.Field{Key: "err_msg", Value: err},
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "url", Value: url},
		)

		return nil, fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	// Добавление стандартных заголовков
	// для всех запросов к API remnawave
	request.Header.Add("Content-Type", "application/json")
	// Обязательно добавляем ключ RemnaKey чтобы панель пропустила
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	// Выполняет запрос
	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to execute doRequest",
			logger.Field{Key: "err_msg", Value: err},
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "url", Value: url},
		)

		return nil, fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	return response, nil
}

// readBody читает тело ответа с логированием ошибок.
func (c *RemnaClient) readBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error(
			"failed to read response body",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
		)

		return nil, fmt.Errorf("%w: %w", ErrFailedToReadBody, err)
	}

	return body, nil
}

// closeBody безопасно закрывает тело ответа с логированием.
func (c *RemnaClient) closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		c.logger.Error(
			"failed to close response body",
			logger.Field{Key: "error", Value: err.Error()},
		)
	}
}

// parseJSON парсит JSON с логированием при ошибке.
func (c *RemnaClient) parseJSON(data []byte, target interface{}) error {
	if err := json.Unmarshal(data, target); err != nil {
		c.logger.Error(
			"failed to unmarshal JSON",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "response_body", Value: string(data)},
		)

		return fmt.Errorf("%w: %w", ErrFailedToUnmarshal, err)
	}

	return nil
}

// isSuccess проверяет 2xx статусы.
func isSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

// handleBasic базовые HTTP запросы.
func (c *RemnaClient) handleBasic(response *http.Response, bodyStr string) (string, error) {
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusInternalServerError, http.StatusNotFound:
		c.logger.Error(
			"в запросе ошибка 400",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrInternalServerError
	}

	return bodyStr, nil
}

// handleReadAndParse обработка с парсингом JSON.
func (c *RemnaClient) handleReadAndParse(
	response *http.Response,
	bodyStr string,
	target interface{},
) (string, error) {
	switch response.StatusCode {
	case http.StatusBadRequest:
		c.logger.Error(
			"bad doRequest",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrBadRequest

	case http.StatusNotFound:
		c.logger.Error(ErrNotFound.Error())

		return bodyStr, ErrNotFound
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrInternalServerError
	}

	// Если статус OK, то парсим
	if response.StatusCode == http.StatusOK && target != nil {
		if err := json.Unmarshal([]byte(bodyStr), target); err != nil {
			c.logger.Error(
				"не удалось распарсить json",
				logger.Field{Key: "status_code", Value: response.StatusCode},
				logger.Field{Key: "response_body", Value: bodyStr},
			)

			return bodyStr, fmt.Errorf("%w: %w", ErrFailedToUnmarshal, err)
		}
	}

	return bodyStr, nil
}

// handleUpdate обработка PATCH/PUT запросов.
func (c *RemnaClient) handleUpdate(response *http.Response, bodyStr string) (string, error) {
	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return bodyStr, nil
	case http.StatusBadRequest:
		c.logger.Error(
			"bad doRequest while updating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrBadRequest
	case http.StatusNotFound:
		c.logger.Error(
			"not found while updating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrNotFound
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error while updating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrInternalServerError
	default:
		c.logger.Error(
			"unexpected status code while updating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
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
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrBadRequestCreate
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error while creating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, ErrInternalServerError
	default:
		c.logger.Error(
			"unexpected status code while creating",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return bodyStr, fmt.Errorf("%w: %d", ErrUndefined, response.StatusCode)
	}
}

// GetUUIDByUsername - метод нахождения пользователя через username.
func (c *RemnaClient) GetUUIDByUsername(ctx context.Context, username string) (string, error) {
	defer c.logDuration("GetUUIDByUsername")()

	var userData models.GetUUIDByUsernameResponse

	// Формируем URL для запроса информации о пользователе по username
	// /api/users/by-username/{username}
	url := fmt.Sprintf(
		"%s/api/users/by-username/%s?%s",
		c.cfg.RemnaPanelURL,
		username,
		c.cfg.RemnaSecretURLToken,
	)

	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	defer c.closeBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", ErrNotFound
	default:
		body, readErr := c.readBody(resp)
		if readErr != nil {
			c.logger.Error("failed to read error response")

			body = []byte{} // пустой слайс вместо nil
		}

		c.logger.Error(
			"ошибка чтения статуса",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response", Value: string(body)},
		)

		return "", ErrInternalServerError
	}

	// При успешном 200 OK читаем дальше и парсим
	// Читаем тело ответа
	body, err := c.readBody(resp)
	if err != nil {
		return "", err
	}

	// Парсим JSON в структуру
	if err := c.parseJSON(body, &userData); err != nil {
		return "", err
	}

	// Проверка есть ли что-то в ответе
	if userData.Response.UUID == "" || userData.Response.Username == "" {
		c.logger.Error(
			"received empty UUID or username in response",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "response_uuid", Value: userData.Response.UUID},
			logger.Field{Key: "response_username", Value: userData.Response.Username},
		)

		return "", ErrUUIDorUsernameIsNill
	}

	c.logger.Info(
		"получен UUID пользователя",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "uuid", Value: userData.Response.UUID},
	)

	// Возвращаем UUID из структуры
	return userData.Response.UUID, nil
}

// SetDevices устанавилвает кол-во устройств пользователя.
func (c *RemnaClient) SetDevices(username string, devices *uint8) error {
	if devices == nil {
		return ErrDevicesNotSet
	}

	defer c.logDuration("SetDevices")()

	// Отправляем только то что нужно изменить, без идентификаторов в теле
	userData := &models.UpdateUserRequest{
		Username:        &username,
		HwidDeviceLimit: devices,
	}

	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	jsonData, err := json.Marshal(userData)
	if err != nil {
		c.logger.Error(
			"failed to marshal doRequest",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		c.logger.Error(
			"failed to make doRequest",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer func() {
		if response != nil {
			if err := response.Body.Close(); err != nil {
				c.logger.Error(
					"failed to close response body",
					logger.Field{Key: "err_msg", Value: err},
				)
			}
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		c.logger.Info(
			fmt.Sprintf("devices for user: %s set successfully", username),
			logger.Field{Key: "status code", Value: response.StatusCode},
		)

		return nil

	case http.StatusBadRequest:
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToReadBody, err)
		}

		c.logger.Error(
			"failed to set devices",
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response body", Value: body},
		)

		return fmt.Errorf("%w: %s", ErrBadRequest, string(body))

	case http.StatusInternalServerError:
		c.logger.Error(
			"failed to set devices",
			logger.Field{Key: "status code", Value: response.StatusCode},
		)

		return fmt.Errorf("%w: %d", ErrInternalServerError, response.StatusCode)

	default:
		return fmt.Errorf("%w: %d", ErrUndefined, response.StatusCode)
	}
}

func (c *RemnaClient) BetterResetTraffic(ctx context.Context, username string) error {
	defer c.logDuration("BetterResetTraffic")()

	UUID, err := c.GetUUIDByUsername(ctx, username)

	switch err {
	case nil:
		c.logger.Info(
			"UUID получен успешно",
		)
	case ErrNotFound:
		c.logger.Error(
			"User not found",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "err_msg", Value: err},
		)

		return ErrNotFound
	case ErrInternalServerError:
		c.logger.Error(
			"Internal server error",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "err_msg", Value: err},
		)
	}

	url := fmt.Sprintf(
		"%s/api/users/%s/actions/reset-traffic?%s",
		c.cfg.RemnaPanelURL,
		UUID,
		c.cfg.RemnaSecretURLToken,
	)

	request, err := http.NewRequestWithContext(ctx, "POST", url, http.NoBody)
	if err != nil {
		return ErrFailedToMakeRequest
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return ErrFailedToDoRequest
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			c.logger.Error(
				"Failed to close response body",
				logger.Field{Key: "username", Value: username},
				logger.Field{Key: "err_msg", Value: err},
			)
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		c.logger.Info(
			"Traffic reset successfully",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: response.Body},
		)

		return nil
	case http.StatusNotFound:
		c.logger.Error(
			"User not found",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: response.Body},
		)

		return ErrNotFound

	case http.StatusBadRequest:
		c.logger.Error(
			"Bad doRequest",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status code", Value: response.StatusCode},
		)

		return ErrBadRequest

	case http.StatusInternalServerError:
		c.logger.Error(
			"Internal server error",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: response.Body},
		)

		return ErrInternalServerError

	default:
		c.logger.Error(
			"Unexpected status code",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: response.Body},
		)

		return ErrUndefined
	}
}

// SetTraffic устанавливает новое значение трафика.
func (c *RemnaClient) SetTraffic(username string, gb uint64) error {
	defer c.logDuration("SetTraffic")()

	// Преобразуем в гиги за шаги
	const bytesInGB uint64 = 1024 * 1024 * 1024

	trafficLimitBytes := gb * bytesInGB

	// Формируем запрос на изменение лимита трафика
	userData := &models.UpdateUserRequest{
		Username:          &username,
		TrafficLimitBytes: &trafficLimitBytes,
	}

	url := fmt.Sprintf(
		"%s/api/users?%s",
		c.cfg.RemnaPanelURL,
		c.cfg.RemnaSecretURLToken,
	)

	// Делаем json запись
	jsonData, err := json.Marshal(userData)
	if err != nil {
		c.logger.Error(
			"ошибка marshal json",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		c.logger.Error(
			"failed to make doRequest",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToDoRequest, err) // ← исправлено
	}

	defer func() {
		if response != nil {
			if err := response.Body.Close(); err != nil {
				c.logger.Error(
					"failed to close response body",
					logger.Field{Key: "err_msg", Value: err},
				)
			}
		}
	}()

	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusInternalServerError:
		return ErrInternalServerError
	case http.StatusBadRequest:
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return ErrReadBody
		}

		c.logger.Error(
			"bad doRequest while setting traffic",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return fmt.Errorf("%w: %s", ErrBadRequestCreate, string(body))
	default:
		body, _ := io.ReadAll(response.Body)

		return fmt.Errorf("%w %d: %s", ErrUndefined, response.StatusCode, string(body))
	}
}

// CreateUser создает пользователя в панели.
func (c *RemnaClient) CreateUser(username string, days int) error {
	defer c.logDuration("CreateUser")()

	if days <= 0 {
		return ErrDaysNotNill
	}

	now := time.Now().UTC()

	// указываем лимиты трафика
	const oneGb int = 1024 * 1024 * 1024
	// лимит 100 gb
	trafficLimit := 100 * oneGb

	// Заполняем структуру для remnawave, чтобы она указала параметры в панели.
	userData := &models.CreateRequestUserDTO{
		Username:             username,
		Status:               "ACTIVE",
		TrojanPassword:       newShortSecret(), // Пароль для протокола Trojan.
		VLessUUID:            uuid.NewString(),
		SsPassword:           newShortSecret(), // Пароль для shadow socks.
		ShortUUID:            newShortSecret(), // Короткий uuid для идентификации.
		TrafficLimitBytes:    trafficLimit,     // Устанавливаем лимит трафика.
		TrafficLimitStrategy: "MONTH",          // Период сброса трафика.
		ExpireAt:             now.AddDate(0, 0, days).Format(time.RFC3339),
		CreatedAt:            now.Format(time.RFC3339),
		LastTrafficResetAt:   now.Format(time.RFC3339),
		Description:          "Created via ProxyMaster",
		ActiveInternalSquads: []string{c.cfg.RemnaSquadUUID},
	}

	// формируем строку куда идет запрос
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	// время для логирования
	start := time.Now()

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			return
		}
	}()

	switch response.StatusCode {
	case http.StatusBadRequest:
		body, err := io.ReadAll(response.Body)
		if err != nil {
			slog.Warn("не удалось преобразовать тело ответа")
		}

		// Проверяем, является ли ошибка "User username already exists"
		if strings.Contains(string(body), "User username already exists") {
			slog.Info("Пользователь уже существует, пропускаем создание", "username", username)

			return nil
		}

		slog.Error(fmt.Sprintf("%s\n%s", ErrBadRequestCreate.Error(), string(body)))

		return ErrBadRequestCreate
	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())

		return ErrInternalServerError
	}

	slog.Info(
		"User created",
		"time taken", time.Since(start),
		"status code", http.StatusCreated,
		"username", userData.Username,
	)

	return nil
}

// ExtendClientSubscription продлевает подписку в панели.
//
//nolint:lll
func (c *RemnaClient) ExtendClientSubscription(userUUID, username string, days int) error {
	defer c.logDuration("ExtendClientSubscription")()

	// формирует url для запроса в api с секретным token для прохода через Nginx
	url := fmt.Sprintf(
		"%s/api/users/bulk/extend-expiration-date?%s",
		c.cfg.RemnaPanelURL,
		c.cfg.RemnaSecretURLToken,
	)

	payload := models.BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	// создаем запрос
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	request.Header.Add("Content-Type", "application/json")

	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	// делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			return
		}
	}()

	// Если соединение прошло, то все отлично.
	if response.StatusCode == http.StatusOK {
		log.Printf(
			"remnawave: период подписки клиента: %s. UUID: %s увеличен на %d дней.\n",
			username,
			userUUID,
			days,
		)
	} else {
		// Если соединения нету.
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("remnawave: не удалось преобразовать тело ответа")

			return ErrFailedToReadBody
		}

		log.Printf(
			"remnawave: не удалось увеличить период подписки клиента: %s. UUID: %s. Тело ошибки: %s.\n",
			username,
			userUUID,
			string(body),
		)

		return ErrFailedToIncreaseSubscriptionPeriod
	}

	return nil
}

// EnableClient включает клиента в панели remnawave.
func (c *RemnaClient) EnableClient(userUUID string) error {
	defer c.logDuration("EnableClient")()

	if err := c.changeUserState(userUUID, "enable"); err != nil {
		return err
	}

	slog.Info("Пользователь успешно включен")

	return nil
}

// DisableClient выключает подписку в панели.
func (c *RemnaClient) DisableClient(userUUID string) error {
	if err := c.changeUserState(userUUID, "disable"); err != nil {
		return err
	}

	slog.Info("Пользователь успешно выключен")

	return nil
}

// AddTraffic добавляет траффик к текущему.
func (c *RemnaClient) AddTraffic(username string, gb uint64) error {
	defer c.logDuration("AddTraffic")()

	user, err := c.GetUserInfo(username)
	if err != nil {
		c.logger.Error(
			"failed to add traffic",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "GB", Value: gb},
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	userTraffic := user.Response.TrafficLimitBytes / (1024 * 1024 * 1024)
	if err = c.SetTraffic(username, (uint64(userTraffic) + gb)); err != nil {
		c.logger.Error(
			"failed to add traffic",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "GB", Value: gb},
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	c.logger.Info(
		"added traffic successfully",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "was GB", Value: userTraffic},
		logger.Field{Key: "added GB", Value: gb},
		logger.Field{Key: "current GB", Value: (uint64(userTraffic) + gb)},
	)

	return nil
}

// GetUserInfo - возвращает информацию.
func (c *RemnaClient) GetUserInfo(uuid string) (models.GetUserInfoResponse, error) {
	defer c.logDuration("GetUserInfo")()

	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.RemnaPanelURL, uuid, c.cfg.RemnaSecretURLToken)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf(
			"remnaClient.GetUserInfo: NewRequestError: %w",
			err,
		)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf(
			"remnaClient.GetUserInfo: GetResponseError: %w",
			err,
		)
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			return
		}
	}()

	switch resp.StatusCode {
	case http.StatusNotFound:
		slog.Error(ErrNotFound.Error())

		return models.GetUserInfoResponse{}, ErrNotFound

	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())

		return models.GetUserInfoResponse{}, ErrInternalServerError

	case http.StatusBadRequest:
		slog.Error(ErrBadRequestUUID.Error())

		return models.GetUserInfoResponse{}, ErrBadRequestUUID
	}

	var userInfo models.GetUserInfoResponse

	// возвращение пустой структуры и ошибки
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.GetUserInfoResponse{}, ErrReadBody
	}

	// возвращение пустой структуры и ошибки
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return models.GetUserInfoResponse{}, ErrUnmarshal
	}

	return userInfo, nil
}

// GetUserStatus возвращает статус пользователя по его UUID.
func (c *RemnaClient) GetUserStatus(uuid string) (string, error) {
	defer c.logDuration("GetUserStatus")()

	userInfo, err := c.GetUserInfo(uuid)
	if err != nil {
		return "", err
	}

	return userInfo.Response.Status, nil
}

func newShortSecret() string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(raw) <= 31 {
		return raw
	}

	return raw[:31]
}

// changeUserState изменяет состояние пользователя в панели Remnawave.
func (c *RemnaClient) changeUserState(userUUID, action string) error {
	url := c.actionURL(userUUID, action)

	c.logger.Debug(
		"trying action",
		logger.Field{Key: "userUUID", Value: userUUID},
		logger.Field{Key: "action", Value: action},
		logger.Field{Key: "url", Value: url},
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, url, http.NoBody)
	if err != nil {
		c.logger.Error(
			"failed to create doRequest for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"failed to execute action doRequest",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToDoRequest, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			return
		}
	}()

	// Читаем тело ответа для логирования
	body, _ := io.ReadAll(resp.Body)

	c.logger.Debug(
		"action response",
		logger.Field{Key: "action", Value: action},
		logger.Field{Key: "status_code", Value: resp.StatusCode},
		logger.Field{Key: "response_body", Value: string(body)},
	)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		c.logger.Info(
			"action executed successfully",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "userUUID", Value: userUUID},
		)

		return nil
	case http.StatusNotFound:
		c.logger.Warn(
			"user not found for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "userUUID", Value: userUUID},
		)

		return ErrNotFound
	case http.StatusInternalServerError:
		c.logger.Error(
			"internal server error for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "userUUID", Value: userUUID},
		)

		return ErrInternalServerError
	case http.StatusBadRequest:
		c.logger.Warn(
			"bad doRequest for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "userUUID", Value: userUUID},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return ErrBadRequestUUID
	default:
		c.logger.Warn(
			"unexpected status code for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "userUUID", Value: userUUID},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return fmt.Errorf("unexpected status code %d for action %s", resp.StatusCode, action)
	}
}

// actionURL отдаем методам строку, чтобы избежать дублирование кода в методах.
func (c *RemnaClient) actionURL(userUUID string, action string) string {
	return fmt.Sprintf("%s/api/users/%s/actions/%s?%s",
		c.cfg.RemnaPanelURL,
		userUUID,
		action,
		c.cfg.RemnaSecretURLToken,
	)
}

// logDuration логирует время выполнения метода.
func (c *RemnaClient) logDuration(method string) func() {
	start := time.Now()

	return func() {
		c.logger.Info("вызов метода завершен",
			logger.Field{Key: "method", Value: method},
			logger.Field{Key: "duration", Value: time.Since(start)},
		)
	}
}

func (c *RemnaClient) DeleteUser(ctx context.Context, username string) error {
	defer c.logDuration("DeleteUser")()

	if username == "" {
		err := fmt.Errorf("указан пустой username для удаления пользователя")
		c.logger.Error(
			"указан пустой username для удаления пользователя",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "err_msg", Value: err},
		)

		return err
	}

	c.logger.Info(
		"starting user deletion",
		logger.Field{Key: "username", Value: username},
	)

	// Получаем UUID пользователя по username
	UUID, err := c.GetUUIDByUsername(ctx, username)
	if err != nil {
		c.logger.Error(
			"failed to get UUID for user deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("failed to get UUID: %w", err)
	}

	c.logger.Debug(
		"UUID obtained for deletion",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "UUID", Value: UUID},
	)

	// Формируем URL для удаления пользователя (не забываем добавить секретный токен)
	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.RemnaPanelURL, UUID, c.cfg.RemnaSecretURLToken)

	// Создаём HTTP-запрос
	request, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, nil)
	if err != nil {
		c.logger.Error(
			"failed to create delete doRequest",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "url", Value: url},
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("failed to create doRequest: %w", err)
	}

	// Добавляем необходимые заголовки
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	// Выполняем запрос
	resp, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to execute delete doRequest",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("failed to execute doRequest: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	// Читаем тело ответа для более подробного логирования ошибок
	respBody, _ := io.ReadAll(resp.Body)

	// Обрабатываем ответ
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		c.logger.Info(
			"user successfully deleted",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
		)

		return nil

	case http.StatusNotFound:
		c.logger.Warn(
			"user not found during deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response_body", Value: string(respBody)},
		)

		return fmt.Errorf("user not found: %s", username)

	case http.StatusUnauthorized:
		c.logger.Error(
			"authorization failed during user deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response_body", Value: string(respBody)},
		)

		return fmt.Errorf("authorization error - check authentication token")

	default:
		c.logger.Error(
			"unexpected status code during user deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: UUID},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response_body", Value: string(respBody)},
		)

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}
