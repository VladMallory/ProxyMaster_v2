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

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
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
		username := "заглушка" // заглушка для wrapErr
		c.wrapErr(err, "Do", username, url)
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
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
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: body},
		)

		return "", fmt.Errorf("%w: %d", ErrBadStatusCode, response.StatusCode)
	}

	body, err := c.readBody(response)
	if err != nil {
		return "", err
	}

	var encResponse models.EncryptURLResponse

	err = c.parseJSON(body, &encResponse)
	if err != nil {
		return "", err
	}

	if encResponse.EncryptedLink == "" {
		c.logger.Error(
			"ошибка получения зашифрванной подписки",
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
	body any,
) (*http.Response, error) {
	var bodyReader io.Reader

	// Преобразуем запрос в JSON чтобы дать
	// его на сервер в привычном виде
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
func (c *RemnaClient) readBody(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error(
			"failed to read response body",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "status_code", Value: resp.StatusCode},
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
			logger.Field{Key: "error", Value: err.Error()},
		)
	}
}

// parseJSON парсит JSON с логированием при ошибке.
func (c *RemnaClient) parseJSON(data string, target any) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		c.logger.Error(
			"failed to unmarshal JSON",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "response_body", Value: data},
		)

		return fmt.Errorf("%w: %w", ErrFailedToUnmarshal, err)
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
	// userData models.GetUUIDByUsernameResponse,
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
			logger.Field{Key: "status_code", Value: resp.StatusCode},
			logger.Field{Key: "response", Value: respBody},
			logger.Field{Key: "field", Value: logField},
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

// handleDelete обрабатывает ответ DELETE запроса.
func (c *RemnaClient) handleDelete(response *http.Response, bodyStr, username, uuid string) error {
	switch response.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		c.logger.Info(
			"user successfully deleted",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: uuid},
			logger.Field{Key: "status_code", Value: response.StatusCode},
		)

		return nil

	case http.StatusNotFound:
		c.logger.Warn(
			"user not found during deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: uuid},
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return ErrNotFound

	case http.StatusUnauthorized:
		c.logger.Error(
			"authorization failed during user deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: uuid},
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return errors.New("authorization error - check authentication token")

	default:
		c.logger.Error(
			"unexpected status code during user deletion",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "UUID", Value: uuid},
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: bodyStr},
		)

		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}
}

// wrapErr убирает дублирование обработки ошибок.
func (c *RemnaClient) wrapErr(err error, msg, username string, url ...string) error {
	if err == nil {
		return nil
	}

	fields := []logger.Field{
		{Key: "username", Value: username},
		{Key: "err_msg", Value: err},
	}

	if len(url) > 0 && url[0] != "" {
		fields = append(fields, logger.Field{Key: "url", Value: url[0]})
	}

	c.logger.Error(msg, fields...)

	return fmt.Errorf("%s: %w", msg, err)
}

// changeUserState изменяет состояние пользователя в панели Remnawave.
func (c *RemnaClient) changeUserState(userUUID, action string) error {
	url := c.actionURL(userUUID, action)

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
