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

// EncryptURL метод, который шифрует URL
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
			"failed to make request",
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

// GetUUIDByUsername - метод нахождения пользователя через username.
func (c *RemnaClient) GetUUIDByUsername(username string) (string, error) {
	defer c.logDuration("GetUUIDByUsername")()

	var userData models.GetUUIDByUsernameResponse
	// /api/users/by-username/{username}
	url := fmt.Sprintf(
		"%s/api/users/by-username/%s?%s",
		c.cfg.RemnaPanelURL,
		username,
		c.cfg.RemnaSecretURLToken,
	)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		url,
		http.NoBody,
	)
	if err != nil {
		slog.Error(err.Error())

		return "", fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	defer func() {
		if err = response.Body.Close(); err != nil {
			c.logger.Error(
				"не удалось закрыть тело ответа",
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error(
			"не удалось преобразовать тело ответа",
			logger.Field{Key: "error", Value: err.Error()},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToMakeResponse, err)
	}

	if err := json.Unmarshal(body, &userData); err != nil {
		c.logger.Error(
			"не удалось распарсить тело ",
			logger.Field{Key: "error", Value: err.Error()},
		)

		return "", fmt.Errorf("%w: %w", ErrFailedToUnmarshal, err)
	}

	switch response.StatusCode {
	case http.StatusBadRequest:
		c.logger.Error(fmt.Sprintf("%s\n%s", ErrBadRequestUsername.Error(), string(body)))

		return "", ErrBadRequestUsername

	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())

		return "", ErrInternalServerError

	case http.StatusNotFound:
		c.logger.Error(ErrNotFound.Error())

		return "", ErrNotFound
	}

	// AI: Защита от некорректных данных : Даже если сервер ответил 200 OK,
	// внутри JSON могут прийти пустые поля (например, если на сервере
	// RemnaWave произошел сбой логики, но не HTTP-ошибка).
	if userData.Response.UUID == "" || userData.Response.Username == "" {
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

// SetDevices устанавилвает кол-во устройств пользователя
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
			"failed to marshal request",
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
			"failed to make request",
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

func (c *RemnaClient) ResetTraffic(username string) error {
	defer c.logDuration("ResetTraffic")()

	c.logger.Info(
		"starting traffic reset",
		logger.Field{Key: "username", Value: username},
	)

	// Сначала попробуем использовать action reset-traffic через UUID
	uuid, err := c.GetUUIDByUsername(username)
	if err != nil {
		c.logger.Error(
			"failed to get UUID for username",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("failed to get UUID: %w", err)
	}

	c.logger.Info(
		"got UUID for user",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "uuid", Value: uuid},
	)

	// Пробуем различные actions для сброса трафика
	possibleActions := []string{
		"reset-traffic",
		"reset-traffic-counter",
		"clear-traffic",
		"reset-usage",
		"reset-traffic-usage",
	}

	c.logger.Info(
		"trying actions for traffic reset",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "actions", Value: strings.Join(possibleActions, ", ")},
	)

	for _, action := range possibleActions {
		c.logger.Info(
			"trying action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "username", Value: username},
		)
		actionErr := c.changeUserState(uuid, action)
		if actionErr == nil {
			c.logger.Info(
				"traffic reset via action successfully",
				logger.Field{Key: "username", Value: username},
				logger.Field{Key: "uuid", Value: uuid},
				logger.Field{Key: "action", Value: action},
			)
			return nil
		}
		c.logger.Warn(
			"action failed",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "err_msg", Value: actionErr},
			logger.Field{Key: "username", Value: username},
		)
	}

	// Если ни один action не сработал, пробуем старый метод PATCH
	c.logger.Info(
		"all traffic reset actions failed, trying PATCH method",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "uuid", Value: uuid},
	)

	now := time.Now().UTC()
	zeroTraffic := uint64(0)

	// Пробуем два варианта: через userTraffic объект и через корневое поле
	userData := map[string]interface{}{
		"username":           username,
		"usedTrafficBytes":   zeroTraffic,
		"lastTrafficResetAt": now,
		"userTraffic": map[string]interface{}{
			"usedTrafficBytes":         zeroTraffic,
			"lifetimeUsedTrafficBytes": zeroTraffic,
		},
	}

	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	jsonData, err := json.Marshal(userData)
	if err != nil {
		c.logger.Error(
			"failed to marshal request",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	c.logger.Info(
		"sending reset traffic request",
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "request_body", Value: string(jsonData)},
	)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		url,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		c.logger.Error(
			"failed to make request",
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

	// Читаем тело ответа для логирования
	var responseBody []byte
	if response.Body != nil {
		responseBody, _ = io.ReadAll(response.Body)
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		c.logger.Info(
			"traffic reset via PATCH successfully",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "status_code", Value: response.StatusCode},
		)

		if len(responseBody) > 0 {
			c.logger.Debug(
				"response body",
				logger.Field{Key: "response_body", Value: string(responseBody)},
			)
		}

		return nil

	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusInternalServerError:
		return ErrInternalServerError
	case http.StatusBadRequest:
		c.logger.Error(
			"bad request while resetting traffic",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: string(responseBody)},
		)

		return fmt.Errorf("%w: %s", ErrBadRequest, string(responseBody))
	default:
		return fmt.Errorf("%w %d: %s", ErrUndefined, response.StatusCode, string(responseBody))
	}
}

// SetTraffic устанавливает новое значение трафика
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

	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

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
			"failed to make request",
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
			"bad request while setting traffic",
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

// AddTraffic добавляет траффик к текущему
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

// GetUserStatus возвращает статус пользователя по его UUID
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
			"failed to create request for action",
			logger.Field{Key: "action", Value: action},
			logger.Field{Key: "err_msg", Value: err},
		)
		return fmt.Errorf("%w: %w", ErrFailedToMakeRequest, err)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"failed to execute action request",
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
			"bad request for action",
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
