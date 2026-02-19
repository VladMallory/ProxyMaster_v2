package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
	"github.com/google/uuid"
)

// CreateUserDTO для передачи данных при создании пользователя.
type CreateUserDTO struct {
	Username string // Имя пользователя в панели
	Days     int    // Количество дней подписки
	FullName string // Полное имя пользователя
	Telegram string // @ телеграм пользователя
}

// CreateUser создает пользователя в панели.
func (c *RemnaClient) CreateUser(dto CreateUserDTO) error {
	defer c.logDuration("CreateUser")()

	if dto.Days <= 0 {
		return ErrDaysNotNill
	}

	description := fmt.Sprintf("Created via ProxyMaster | %s | @%s", dto.FullName, dto.Telegram)

	now := time.Now().UTC()

	// указываем лимиты трафика
	const oneGb int = 1024 * 1024 * 1024
	// лимит 100 gb
	trafficLimit := 100 * oneGb

	// Заполняем структуру для remnawave, чтобы она указала параметры в панели.
	userData := &models.CreateRequestUserDTO{
		Username:             dto.Username,
		Status:               "ACTIVE",
		TrojanPassword:       newShortSecret(), // Пароль для протокола Trojan.
		VLessUUID:            uuid.NewString(),
		SsPassword:           newShortSecret(), // Пароль для shadow socks.
		ShortUUID:            newShortSecret(), // Короткий uuid для идентификации.
		TrafficLimitBytes:    trafficLimit,     // Устанавливаем лимит трафика.
		TrafficLimitStrategy: "MONTH",          // Период сброса трафика.
		ExpireAt:             now.AddDate(0, 0, dto.Days).Format(time.RFC3339),
		CreatedAt:            now.Format(time.RFC3339),
		LastTrafficResetAt:   now.Format(time.RFC3339),
		Description:          description,
		ActiveInternalSquads: []string{c.cfg.RemnaSquadUUID},
	}

	// формируем строку куда идет запрос
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

	resp, err := c.doRequest(context.Background(), http.MethodPost, url, userData)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	body, err := c.readBody(resp)
	if err != nil {
		return err
	}

	if _, err := c.handleCreate(resp, body); err != nil {
		return err
	}

	c.logger.Info(
		"успешное создание пользователя в панели",
		logger.Field{Key: "username", Value: dto.Username},
	)

	return nil
}

// ExtendClientSubscription продлевает подписку в панели.
func (c *RemnaClient) ExtendClientSubscription(userUUID, username string, days int) error {
	defer c.logDuration("ExtendClientSubscription")()

	url := fmt.Sprintf(
		"%s/api/users/bulk/extend-expiration-date?%s",
		c.cfg.RemnaPanelURL,
		c.cfg.RemnaSecretURLToken,
	)

	payload := models.BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}

	resp, err := c.doRequest(context.Background(), http.MethodPost, url, payload)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	if resp.StatusCode == http.StatusOK {
		c.logger.Info(
			"Период подписки продлён",
			logger.Field{Key: "username", Value: username},
			logger.Field{Key: "user_uuid", Value: userUUID},
			logger.Field{Key: "days", Value: days},
		)

		return nil
	}

	_, err = c.readBody(resp)
	if err != nil {
		return err
	}

	return c.wrapErr(nil, "Failed to increase subscription period", username)
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

// GetUUIDByUsername - метод нахождения пользователя через username.
func (c *RemnaClient) GetUUIDByUsername(ctx context.Context, username string) (string, error) {
	defer c.logDuration("GetUUIDByUsername")()

	// Формируем URL для запроса информации о пользователе по username
	// /api/users/by-username/{username}
	url := fmt.Sprintf(
		"%s/api/users/by-username/%s?%s",
		c.cfg.RemnaPanelURL,
		username,
		c.cfg.RemnaSecretURLToken,
	)

	var userData models.GetUUIDByUsernameResponse

	// Выполняем HTTP запрос и парсим ответ через вспомогательный метод
	if err := c.doRequestAndParse(
		ctx,
		http.MethodGet,
		url,
		nil,
		&userData,
		username); err != nil {
		return "", err
	}

	// Проверка валидности ответа
	if userData.Response.UUID == "" || userData.Response.Username == "" {
		c.logger.Error(
			"received empty UUID or username in response",
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

// DeleteUser удаляет клиенту из панели.
func (c *RemnaClient) DeleteUser(ctx context.Context, username string) error {
	defer c.logDuration("DeleteUser")()

	if username == "" {
		err := errors.New("указан пустой username для удаления пользователя")

		return err
	}

	// Получаем UUID пользователя по username
	UUID, err := c.GetUUIDByUsername(ctx, username)
	if err != nil {
		c.wrapErr(err, "GetUUIDByUsername", username)

		return fmt.Errorf("failed to get UUID: %w", err)
	}

	// Формируем URL для удаления пользователя (не забываем добавить секретный токен)
	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.RemnaPanelURL, UUID, c.cfg.RemnaSecretURLToken)

	resp, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	respBody, _ := c.readBody(resp)

	return c.handleDelete(resp, respBody, username, UUID)
}
