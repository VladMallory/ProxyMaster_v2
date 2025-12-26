package remnawave

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/models"
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

type RemnaClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewRemnaClient конструктор для создания клиента.
func NewRemnaClient(cfg *config.Config) *RemnaClient {
	return &RemnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Хорошая практика: всегда задавать таймаут
		},
	}
}

// GetUUIDByUsername - метод нахождения пользователя через UUID
func (c *RemnaClient) GetUUIDByUsername(username string) (string, error) {
	var userData models.GetUUIDByUsernameResponse
	// /api/users/by-username/{username}
	url := fmt.Sprintf("%s/api/users/by-username/%s?%s", c.cfg.RemnaPanelURL, username, c.cfg.RemnaSecretUrlToken)
	rt := time.Now()

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Error("не удалось получить ответ")
		return "", err
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Warn("не удалось преобразовать тело ответа")
	}

	if err := json.Unmarshal(body, &userData); err != nil {
		slog.Error("не удалось заанмаршалить тело ответа")
		return "", err
	}

	switch response.StatusCode {

	case http.StatusBadRequest:
		slog.Error(fmt.Sprintf("%s\n%s", ErrBadRequestUsername.Error(), string(body)))
		return "", ErrBadRequestUsername

	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())
		return "", ErrInternalServerError

	case http.StatusNotFound:
		slog.Error(ErrNotFound.Error())
		return "", ErrNotFound
	}

	slog.Info(
		"getting UUID succeeded",
		"time taken", time.Since(rt),
		"status code", response.StatusCode,
		"username", userData.Response.Username,
	)

	// проверка что не nil ответ, дабы не повторять что было
	if userData.Response.UUID == "" || userData.Response.Username == "" {
		return "", fmt.Errorf("UUID or Username Is nil")
	}

	return userData.Response.UUID, nil
}

// CreateUser создает нового клиента в remnawave. Если уже
// существует клиент, нечего не делает (дает ошибку о неверном запросе)
func (c *RemnaClient) CreateUser(username string, days int) error {
	if days <= 0 {
		return fmt.Errorf("дней не может быть ноль при создании подписки")
	}

	now := time.Now().UTC()

	// указываем лимиты трафика
	const oneGb int = 1024 * 1024 * 1024
	// лимит 100 gb
	var trafficLimit = 100 * oneGb

	// заполняем структуру для remnawave, чтобы она указала параметры в панели
	userData := &models.CreateRequestUserDTO{
		Username:             username,
		Status:               "ACTIVE",
		TrojanPassword:       newShortSecret(), // пароль для протокола trojan
		VLessUUID:            uuid.NewString(),
		SsPassword:           newShortSecret(), // пароль для shadow socks
		TrafficLimitBytes:    trafficLimit,     // устанавливаем лимит трафика
		TrafficLimitStrategy: "MONTH",          // период сброса трафика
		ExpireAt:             now.AddDate(0, 0, days).Format(time.RFC3339),
		CreatedAt:            now.Format(time.RFC3339),
		LastTrafficResetAt:   now.Format(time.RFC3339),
	}

	// формируем строку куда идет запрос
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretUrlToken)

	// время для логирования
	start := time.Now()

	jsonData, err := json.Marshal(userData)
	if err != nil {
		slog.Error("ошибка создания json тела запроса")
		return err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("не удалось создать запрос")
		return err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Error("не удалось получить ответ")
		return err
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

// ExtendClientSubscription продлевает подписку в панели
func (c *RemnaClient) ExtendClientSubscription(userUUID string, days int) error {
	// формирует url для запроса в api с секретным token для прохода через Nginx
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretUrlToken)

	payload := models.BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга тела запроса: %w", err)
	}

	// создаем запрос
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)

	// делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			return
		}
	}()

	// если соединение прошло, то все отлично
	if response.StatusCode == http.StatusOK {
		log.Printf("%s | Период подписки юзера с UUID: %s увеличен на %d дней.\n", time.Now(), userUUID, days)
	} else {
		// ну плохо все
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("Не удалось преобразовать тело ответа")
			return fmt.Errorf("не удалось преобразовать тело ответа")
		}
		log.Printf(
			"%s | Не удалось увеличить период подписки юзера с UUID: %s. Тело ошибки: %s.\n", time.Now(), userUUID, string(body))
		return fmt.Errorf("не удалось увеличить период подписки")
	}
	return nil
}

// actionUrl отдаем методам строку, чтобы избежать дублирование кода в методах
func (c *RemnaClient) actionUrl(userUUID string, action string) string {
	return fmt.Sprintf("%s/api/users/%s/actions/%s?%s",
		c.cfg.RemnaPanelURL,
		userUUID,
		action,
		c.cfg.RemnaSecretUrlToken,
	)
}

// changeUserState изменяет состояние пользователя в панели Remnawave
func (c *RemnaClient) changeUserState(userUUID string, action string) error {
	url := c.actionUrl(userUUID, action)

	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
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

// EnableClient включает клиента в панели remnawave
func (c *RemnaClient) EnableClient(userUUID string) error {
	if err := c.changeUserState(userUUID, "enable"); err != nil {
		return err
	}

	slog.Info("Пользователь успешно включен")
	return nil
}

// DisableClient выключает подписку в панели
func (c *RemnaClient) DisableClient(userUUID string) error {
	if err := c.changeUserState(userUUID, "disable"); err != nil {
		return err
	}

	slog.Info("Пользователь успешно выключен")
	return nil
}

// GetUserInfo - возвращает информацию
func (c *RemnaClient) GetUserInfo(uuid string) (models.GetUserInfoResponse, error) {
	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.RemnaPanelURL, uuid, c.cfg.RemnaSecretUrlToken)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: NewRequestError: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: GetResponseError: %v", err)
	}

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
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: ReadBodyError")
	}

	// возвращение пустой структуры и ошибки
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return models.GetUserInfoResponse{}, fmt.Errorf("remnaClient.GetUserInfo: UnmarshalingError")
	}

	return userInfo, nil
}

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

// ====LEGACY====
// НЕ ИСПОЛЬЗУЕТСЯ

// Deprecated: Login является legacy-методом и сохранён на случай если понадобится снова
// В текущей реализации клиент использует секретный URL,
// поэтому аутентификация не требуется и метод можно не вызывать
func (c *RemnaClient) Login(ctx context.Context, username, password string) error {
	// подготавливаем данные для входа в панель
	reqBody := models.LoginRequest{
		Username: username,
		Password: password,
	}

	// превращаем верхнюю структуру из go кода в json чтобы панель нас поняла
	// {"username":"admin","password":1234"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка кодирования данных: %w", err)
	}

	// делаем итоговую ссылку куда идем логиниться
	// Сохраняем адрес в requestURL поскольку remna
	// разрешает вход только по секретному адресу
	requestURL := fmt.Sprintf("%s/api/auth/login?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretUrlToken)

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
		return fmt.Errorf("ошибка входа: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}

	var loginResp models.LoginResponse
	// получаем пропуск (токен)
	// после получения json преобразуем обратно в go код
	if err = json.Unmarshal(bodyBytes, &loginResp); err != nil {
		return fmt.Errorf("ошибка разбора ответа: %w, тело: %s", err, string(bodyBytes))
	}

	// Сохраняем полученный токен
	c.cfg.RemnawaveKey = loginResp.Response.AccessToken

	// Логирование успеха как в remna.go
	slog.Info(
		"Login is successful",
		"status", resp.Status,
		"response_body", string(bodyBytes),
	)
	return nil
}
