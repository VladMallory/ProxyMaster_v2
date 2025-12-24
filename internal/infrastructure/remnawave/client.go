package remnawave

import (
	"ProxyMaster_v2/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"
)

type RemnaClient struct {
	cfg        *models.Config
	httpClient *http.Client
}

// NewRemnaClient - конструктор для создания клиента.
func NewRemnaClient(cfg *models.Config) *RemnaClient {
	return &RemnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Хорошая практика: всегда задавать таймаут
		},
	}
}

// метод нахождения пользователя через UUID
func (c *RemnaClient) GetUUIDByUsername(username string) (string, error) {
	var userData models.GetUUIDByUsernameResponse
	///api/users/by-username/{username}
	url := fmt.Sprintf("%s/api/users/by-username/%s?%s", c.cfg.BaseURL, username, c.cfg.SecretURLToken)
	rt := time.Now()

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)

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

// TODO: создание клиента
// remnawave.CreateClient("123")

func (c *RemnaClient) CreateClient(userData *models.CreateRequestUserDTO) error {
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.BaseURL, c.cfg.SecretURLToken)
	rt := time.Now()
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
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)

	response, err := c.httpClient.Do(request)
	if err != nil {
		slog.Error("не удалось получить ответ")
		return err
	}
	defer response.Body.Close()

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
		"time taken", time.Since(rt),
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
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date?%s", c.cfg.BaseURL, c.cfg.SecretURLToken)

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
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)

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
	url := fmt.Sprintf("%s/api/users/%s/actions/enable?%s", c.cfg.BaseURL, userUUID, c.cfg.SecretURLToken)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)
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
	url := fmt.Sprintf("%s/api/users/%s/actions/disable?%s", c.cfg.BaseURL, userUUID, c.cfg.SecretURLToken)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)
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
