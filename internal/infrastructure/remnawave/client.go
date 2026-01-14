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
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/pkg/logger"

	"github.com/google/uuid"
)

// RemnaClient описывает то что нужно для работы remnawave.
type RemnaClient struct {
	cfg        *config.Config
	httpClient *http.Client
	// Храним логгер здесь для обращения к нему
	logger logger.Logger
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

		return "", fmt.Errorf("marshal encrypt request error: %w", err)
	}

	apiURL := "https://crypto.happ.su/api.php"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error(
			"failed to make request",
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("create encrypt request error: %w", err)
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("execute encrypt request error: %w", err)
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

			return "", fmt.Errorf("bad status code error: %d, failed to read body: %w", response.StatusCode, readErr)
		}

		c.logger.Error(
			"bad status code",
			logger.Field{Key: "status_code", Value: response.StatusCode},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", fmt.Errorf("bad status code error: %d, body: %s", response.StatusCode, string(body))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error(
			"failed to read response body",
			logger.Field{Key: "err_msg", Value: err},
		)

		return "", fmt.Errorf("read response body error: %w", err)
	}

	var encResponse models.EncryptURLResponse
	if err := json.Unmarshal(body, &encResponse); err != nil {
		c.logger.Error(
			"failed to unmarshal json",
			logger.Field{Key: "err_msg", Value: err},
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", fmt.Errorf("unmarshal encrypt response error: %w", err)
	}

	if encResponse.EncryptedLink == "" {
		c.logger.Error(
			"received empty encrypted link",
			logger.Field{Key: "response_body", Value: string(body)},
		)

		return "", fmt.Errorf("получен пустой URL")
	}

	return encResponse.EncryptedLink, nil
}

// GetUUIDByUsername - метод нахождения пользователя через username.
func (c *RemnaClient) GetUUIDByUsername(username string) (string, error) {
	defer c.logDuration("GetUUIDByUsername")()

	var userData models.GetUUIDByUsernameResponse
	// /api/users/by-username/{username}
	url := fmt.Sprintf("%s/api/users/by-username/%s?%s", c.cfg.RemnaPanelURL, username, c.cfg.RemnaSecretURLToken)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		slog.Error(err.Error())

		return "", fmt.Errorf("remnawave: не удалось создать request: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("remnawave: не удалось получить ответ: %w", err)
	}
	defer func() {
		if err = response.Body.Close(); err != nil {
			c.logger.Error("не удалось закрыть тело ответа", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.logger.Error("не удалось преобразовать тело ответа", logger.Field{Key: "error", Value: err.Error()})

		return "", fmt.Errorf("remnawave: не удалось преобразовать тело ответа: %w", err)
	}

	if err := json.Unmarshal(body, &userData); err != nil {
		c.logger.Error("не удалось распарсить тело ответа", logger.Field{Key: "error", Value: err.Error()})

		return "", fmt.Errorf("remnawave: не удалось распарсить тело ответа: %w", err)
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
		return "", errors.New("UUID or Username Is nil")
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
		return fmt.Errorf("не указано кол-во устройств в методе SetDevices")
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

		return fmt.Errorf("marshal request: %w", err)
	}

	request, err := http.NewRequestWithContext(context.Background(), "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error(
			"failed to make request",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("create request error: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("execute request error: %w", err)
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
			return fmt.Errorf("read response body error: %w", err)
		}
		c.logger.Error(
			"failed to set devices",
			logger.Field{Key: "status code", Value: response.StatusCode},
			logger.Field{Key: "response body", Value: body},
		)

		return fmt.Errorf("bad request error: %s", string(body))

	case http.StatusInternalServerError:
		c.logger.Error(
			"failed to set devices",
			logger.Field{Key: "status code", Value: response.StatusCode},
		)

		return fmt.Errorf("server returned internal error: %v", response.StatusCode)

	default:
		return fmt.Errorf("undefined status code error: %d", response.StatusCode)
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

		return fmt.Errorf("marshal json data error: %w", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error(
			"failed to make request",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("create http request error: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error(
			"failed to get response",
			logger.Field{Key: "err_msg", Value: err},
		)

		return fmt.Errorf("send http request: %w", err) // ← исправлено
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

		return fmt.Errorf("remnawave: unexpected status code %d: %s", response.StatusCode, string(body))
	}
}

// CreateUser создает пользователя в панели.
func (c *RemnaClient) CreateUser(username string, days int) error {
	if days <= 0 {
		return errors.New("дней не может быть ноль при создании подписки")
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
		return fmt.Errorf("remnawave: не удалось создать тело запроса: %w", err)
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("remnawave: не удалось создать request: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("remnawave: не удалось получить ответ: %w", err)
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
	// формирует url для запроса в api с секретным token для прохода через Nginx
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	payload := models.BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга тела запроса: %w", err)
	}

	// создаем запрос
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("remnawave: не удалось создать request: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")

	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	// делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("remnawave: не удалось получить ответ: %w", err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			return
		}
	}()

	// Если соединение прошло, то все отлично.
	if response.StatusCode == http.StatusOK {
		log.Printf("remnawave: период подписки клиента: %s. UUID: %s увеличен на %d дней.\n", username, userUUID, days)
	} else {
		// Если соединения нету.
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("remnawave: не удалось преобразовать тело ответа")

			return errors.New("remnawave: не удалось преобразовать тело ответа")
		}

		log.Printf("remnawave: не удалось увеличить период подписки клиента: %s. UUID: %s. Тело ошибки: %s.\n", username, userUUID, string(body))

		return errors.New("remnawave: не удалось увеличить период подписки")
	}

	return nil
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

// changeUserState изменяет состояние пользователя в панели Remnawave.
func (c *RemnaClient) changeUserState(userUUID, action string) error {
	url := c.actionURL(userUUID, action)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("remnawave: не удалось создать request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("remnawave: не удалось получить ответ: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			return
		}
	}()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusInternalServerError:
		return ErrInternalServerError
	case http.StatusBadRequest:
		return ErrBadRequestUUID
	}

	return nil
}

// EnableClient включает клиента в панели remnawave.
func (c *RemnaClient) EnableClient(userUUID string) error {
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

// GetUserInfo - возвращает информацию.
func (c *RemnaClient) GetUserInfo(uuid string) (models.GetUserInfoResponse, error) {
	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.RemnaPanelURL, uuid, c.cfg.RemnaSecretURLToken)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: NewRequestError: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnaKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: GetResponseError: %w", err)
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
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: %w", ErrReadBody)
	}

	// возвращение пустой структуры и ошибки
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: %w", ErrUnmarshal)
	}

	return userInfo, nil
}

// GetUserStatus возвращает статус пользователя по его UUID
func (c *RemnaClient) GetUserStatus(uuid string) (status string, err error) {
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

// Login выполняет аутентификацию в системе Remnawave
// Должно быть (правильный формат комментария):
// ====LEGACY====
func (c *RemnaClient) Login(ctx context.Context, username, password string) error {
	// Подготавливаем данные для входа в панель.
	reqBody := models.LoginRequest{
		Username: username,
		Password: password,
	}

	// Превращаем верхнюю структуру из go кода в json чтобы панель нас поняла.
	// {"username":"admin","password":1234"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка кодирования данных: %w", err)
	}

	// делаем итоговую ссылку куда идем логиниться
	// Сохраняем адрес в requestURL поскольку remna
	// разрешает вход только по секретному адресу
	requestURL := fmt.Sprintf("%s/api/auth/login?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	// Создаем запрос на отправку. Как письмо перед отправкой.
	// Кладем итоговый путь и json который делали как паспорт при
	// http. MethodPost - даем POST, отправляем данные на сервер
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// Сервер может принимать видео, текст и т.д.
	// Тут мы говорим прям, что даем json
	// Content-Type - указываем какой тип контента
	req.Header.Set("Content-Type", "application/json")

	// вот тут идет коннект с сервером
	// httpClient.Do() отправляет данные и в
	// него кладем что отправим на сервер
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			slog.Error(
				"Ошибка закрытия соединения",
				"error", err,
			)
		}
	}()

	// Читаем что вернул сервер, а вернул он resp. Body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error(
			"ошибка чтения тела ответа",
			"response_body", string(bodyBytes),
		)
	}

	// вернул ли сервер OK (200) или Created (201) иначе ошибка
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		slog.Error(
			"ошибка входа",
			"status_code", resp.StatusCode,
			"response_body", string(bodyBytes),
		)

		return fmt.Errorf("ошибка входа: %d, тело: %s: %w", resp.StatusCode, string(bodyBytes), ErrLoginFailed)
	}

	var loginResp models.LoginResponse
	// получаем пропуск (токен)
	// после получения json преобразуем обратно в go код
	if err = json.Unmarshal(bodyBytes, &loginResp); err != nil {
		return fmt.Errorf("ошибка разбора ответа: %w, тело: %s", err, string(bodyBytes))
	}

	// Сохраняем полученный токен
	c.cfg.RemnaKey = loginResp.Response.AccessToken

	// Логирование успеха как в remna.go
	slog.Info(
		"Login is successful",
		"status", resp.Status,
		"response_body", string(bodyBytes),
	)

	return nil
}
