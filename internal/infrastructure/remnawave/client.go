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

// NewRemnaClient - конструктор для создания клиента.
func NewRemnaClient(cfg *config.Config) *RemnaClient {
	return &RemnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Хорошая практика: всегда задавать таймаут
		},
	}
}

// функция спизженна из старого клиента, на переделанно логирование
func (c *RemnaClient) Login(ctx context.Context, username, password string) error {
	// подготавливаем данные для входа в панель
	reqBody := models.LoginRequest{
		Username: username,
		Password: password,
	}

	// превращаем верхнюю структуру из GOOO кода в json чтобы панель нас поняла
	// {"username":"admin","password":1234"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка кодирования данных: %w", err)
	}

	// делаем итоговую ссылку куда идем логниться
	// Сохраняем адрес в requestURL поскольку remna
	// разрешает вход только по секретному адресу
	requestURL := fmt.Sprintf("%s/api/auth/login?%s", c.cfg.RemnaPanelURL, c.cfg.RemnasecretUrlToken)

	// создаем запрос на отправку. Как письмо перед отправкой.
	// Кладем итоговый путь и json который делали как паспорт при
	// http.MethodPost - даем метом POST, отправляем данные на сервер
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// сервер может принимать видео, текст и т.д.
	// тут мы говорим прям что даем json
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
				"Ошибка закрыьия соединения",
				"error", err,
			)
		}
	}()

	// читаем что вернул сервер, а вернул он resp.Body
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
	// после получения json преобразуем обратно в GOOO код
	if err = json.Unmarshal(bodyBytes, &loginResp); err != nil {
		return fmt.Errorf("ошибка разбора ответа: %w, тело: %s", err, string(bodyBytes))
	}

	// Сохраняем полученный токен
	c.cfg.RemnawaveKey = loginResp.Response.AccessToken

	// Логирование успеха как в remna.go
	slog.Info(
		"Login is succesful",
		"status", resp.Status,
		"response_body", string(bodyBytes),
	)
	return nil
}

func (c *RemnaClient) GetServiceInfo(ctx context.Context, serviceID string) (string, error) {
	// 1. Формируем URL для запроса
	// Если serviceID пустой, запрашиваем список всех сервисов без слэша на конце
	requestURL := fmt.Sprintf("%s/api/services", c.cfg.RemnaPanelURL)
	if serviceID != "" {
		requestURL = fmt.Sprintf("%s/%s", requestURL, serviceID)
	}

	// 2. Создаем новый HTTP запрос с контекстом (важно для отмены запросов)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("не удалось создать запрос: %w", err)
	}

	// 3. Добавляем необходимые заголовки авторизации
	req.Header.Set("Authorization", "Bearer "+c.cfg.RemnawaveKey)
	req.Header.Set("Content-Type", "application/json")

	// 4. Выполняем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	// Важно: всегда закрываем тело ответа, чтобы не было утечек памяти
	defer resp.Body.Close()

	// 5 читаем тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %w", err)
	}
	bodyString := string(bodyBytes)

	// 6 проверяем статус код ответа
	if resp.StatusCode != http.StatusOK {
		// Если это HTML (например 404 от nginx), не нужно выводить все тело
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("ошибка: ресурс не найден (404) по адресу %s", requestURL)
		}
		return "", fmt.Errorf("неккоректный статус ответа: %d, тело: %s", resp.StatusCode, bodyString)
	}

	// проверяю какая вообще ошибка возвращения
	return bodyString, nil
}

// метод нахождения пользователя через UUID
func (c *RemnaClient) GetUUIDByUsername(username string) (string, error) {
	var userData models.GetUUIDByUsernameResponse
	///api/users/by-username/{username}
	url := fmt.Sprintf("%s/api/users/by-username/%s?%s", c.cfg.RemnaPanelURL, username, c.cfg.RemnasecretUrlToken)
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
		"getting UUID succeded",
		"time taken", time.Since(rt),
		"status code", 200,
		"username", userData.Username,
	)
	return userData.UUID, nil
}

// CreateClient - создает нового клиента в remnawave. Если уже
// существует клиент, нечего не делает (сыпит ошибку о неверном запросе)
func (c *RemnaClient) CreateClient(username string, days int) error {
	if days <= 0 {
		return fmt.Errorf("дней не может быть ноль при создании подписки")
	}

	now := time.Now().UTC()

	// указываем лимиты трафика
	const oneGb int = 1024 * 1024 * 1024
	// лимит 100 гигов
	var tratrafficLimitsTotal int = 100 * oneGb

	// заполняем структуру для ремны, чтобы она указала параметры в панели
	userData := &models.CreateRequestUserDTO{
		Username:             username,
		Status:               "ACTIVE",
		TrojanPassword:       newShortSecret(), // пароль для протокола trojan
		VLessUUID:            uuid.NewString(),
		SsPassword:           newShortSecret(), // пароль для shadowsocks
		TrafficLimitBytes:    tratrafficLimitsTotal,
		TrafficLimitStrategy: "MONTH", // период сброса трафика
		ExpireAt:             now.AddDate(0, 0, days).Format(time.RFC3339),
		CreatedAt:            now.Format(time.RFC3339),
		LastTrafficResetAt:   now.Format(time.RFC3339),
	}

	// формируем строку куда идет запрос
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.BaseURL, c.cfg.SecretURLToken)

	// время для логирования
	start := time.Now()

	jsonData, err := json.Marshal(userData)
	if err != nil {
		slog.Error("ошибка маршалинга тела запроса")
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
		"status code", 201,
		"username", userData.Username,
	)
	return nil
}

// TODO: продление подписки. Указываем id и на сколько дней
// remnawave.ExtendClientSubscriptio("123", 30)
// TODO: сделать ветвление статус кодов и переделать логированиеы
func (c *RemnaClient) ExtendClientSubscription(userUUID string, days int) error {
	//формирует url для запроса в api с секретным токеном для прохода через Nginx
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date?%s", c.cfg.RemnaPanelURL, c.cfg.RemnasecretUrlToken)

	payload := models.BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга тела запроса: %w", err)
	}

	//создаем запрос
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)

	//делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	//закрытие тела запроса, хз зачем, в гошке есть сборщик мусора, ну а хули пусть будет
	defer response.Body.Close()

	//если ок, то заебок
	if response.StatusCode == http.StatusOK {
		log.Printf("%s | Период подписки юзера с UUID: %s увеличен на %d дней.\n", time.Now(), userUUID, days)
	} else {
		//ну плохо все
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("Не удалось преобразовать тело ответа")
			return fmt.Errorf("Не удалось преобразовать тело ответа.")
		}
		log.Printf(
			"%s | Не удалось увеличить период подписки юзера с UUID: %s. Тело ошибки: %s.\n", time.Now(), userUUID, string(body))
		return fmt.Errorf("Не удалось увеличить период подписки.")
	}
	return nil
}

// TODO: включение подписки -- сделал
// remnawave.EnableClient("13123")

// Илья/ я сделаю или сделал отдельную функцию перевода username в userUUID
// затести эту функцию пж я не могу сам тестить(не знаю как, но по сути все норм должно быть)
func (c *RemnaClient) EnableClient(userUUID string) error {
	url := fmt.Sprintf("%s/api/users/%s/actions/enable?%s", c.cfg.RemnaPanelURL, userUUID, c.cfg.RemnasecretUrlToken)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)
	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Warn("не удалось сделать запрос")
		return err
	}

	switch response.StatusCode {
	case http.StatusNotFound:
		slog.Error(ErrNotFound.Error())
		return ErrNotFound
	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())
		return ErrInternalServerError
	case http.StatusBadRequest:
		slog.Error(ErrBadRequestUUID.Error())
		return ErrBadRequestUUID
	}
	slog.Info("Пользователь успешно включен")
	return nil
}

// TODO: Выключение подписки - сделал
// remnawave.DisableClient("123")
func (c *RemnaClient) DisableClient(userUUID string) error {
	url := fmt.Sprintf("%s/api/users/%s/actions/disable?%s", c.cfg.RemnaPanelURL, userUUID, c.cfg.RemnasecretUrlToken)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	request.Header.Add("Authorization", "Bearer "+c.cfg.RemnawaveKey)
	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Warn("не удалось сделать запрос")
		return err
	}

	switch response.StatusCode {
	case http.StatusNotFound:
		slog.Error(ErrNotFound.Error())
		return ErrNotFound
	case http.StatusInternalServerError:
		slog.Error(ErrInternalServerError.Error())
		return ErrInternalServerError
	case http.StatusBadRequest:
		slog.Error(ErrBadRequestUUID.Error())
		return ErrBadRequestUUID
	}
	slog.Info("Пользователь успешно выключен")
	return nil
}

// GetServiceInfo - получение информации о сервисе
// Реализует интерфейс domain.RemnawaveClient
func (c *RemnaClient) GetServiceInfo(ctx context.Context, serviceID string) (string, error) {
	// TODO: Здесь должен быть реальный запрос к API Remnawave для получения информации о сервисе.
	// Поскольку эндпоинт неизвестен из текущего контекста, возвращаем ошибку, чтобы не было молчаливого сбоя.
	slog.Info("Запрос информации о сервисе", "serviceID", serviceID)

	return "", fmt.Errorf("метод GetServiceInfo еще не реализован: отсутствует URL эндпоинта")
}

func newShortSecret() string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(raw) <= 31 {
		return raw
	}
	return raw[:31]
}
