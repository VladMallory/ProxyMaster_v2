package remnawave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateUserDTO для передачи данных при создании пользователя.
type CreateUserDTO struct {
	Username    string // Имя пользователя в панели
	Days        int    // Количество дней подписки
	FullName    string // Полное имя пользователя
	Telegram    string // @ телеграм пользователя
	DeviceLimit *int   // Лимит устройств (HWID). nil = из конфига
}

// CreateUser создает пользователя в панели.
func (c *RemnaClient) CreateUser(dto CreateUserDTO) error {
	defer c.logDuration("CreateUser")()

	if dto.Days <= 0 {
		return ErrDaysNotNill
	}

	// Определяем лимит устройств: если nil, отправляем nil в JSON
	var deviceLimit *int
	if dto.DeviceLimit != nil {
		deviceLimit = dto.DeviceLimit
	}

	description := fmt.Sprintf("Created via ProxyMaster | %s | @%s", dto.FullName, dto.Telegram)

	now := time.Now().UTC()

	// указываем лимиты трафика
	const oneGb int64 = 1024 * 1024 * 1024
	trafficLimit := c.cfg.TrafficLimitGB * oneGb

	// Заполняем структуру для remnawave, чтобы она указала параметры в панели.
	userData := &CreateRequestUserDTO{
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
		HWIDDeviceLimit:      deviceLimit, // Устанавливаем лимит устройств.
		ActiveInternalSquads: []string{c.cfg.SquadUUID},
	}

	// формируем строку куда идет запрос
	url := fmt.Sprintf("%s/api/users?%s", c.cfg.PanelURL, c.cfg.SecretURLToken)

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
		zap.String("username", dto.Username),
	)

	return nil
}

// ExtendClientSubscription продлевает подписку в панели.
func (c *RemnaClient) ExtendClientSubscription(userUUID, username string, days int) error {
	defer c.logDuration("ExtendClientSubscription")()

	url := fmt.Sprintf(
		"%s/api/users/bulk/extend-expiration-date?%s",
		c.cfg.PanelURL,
		c.cfg.SecretURLToken,
	)

	// Новый API ждёт массив числовых id (userIds), а не uuids.
	id, err := strconv.ParseInt(userUUID, 10, 64)
	if err != nil {
		return c.wrapErr(err, "не удалось распарсить id пользователя", username)
	}

	payload := BulkExtendRequest{
		UserIDs: []int64{id},
		Days:    days,
	}

	resp, err := c.doRequest(context.Background(), http.MethodPost, url, payload)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	// Новый API отвечает 204 No Content, а не 200 OK.
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		c.logger.Info(
			"Период подписки продлён",
			zap.String("username", username),
			zap.String("user_uuid", userUUID),
			zap.Int("days", days),
		)

		return nil
	}

	_, err = c.readBody(resp)
	if err != nil {
		return err
	}

	return c.wrapErr(
		fmt.Errorf("неизвестный статус: %d", resp.StatusCode),
		"Failed to increase subscription period",
		username,
		url,
	)
}

// EnableClient включает клиента в панели remnawave.
func (c *RemnaClient) EnableClient(userUUID string) error {
	defer c.logDuration("EnableClient")()

	if err := c.changeUserState(userUUID, "enable"); err != nil {
		return err
	}

	c.logger.Info(
		"пользователь успешно включен",
		zap.String("userUUID", userUUID),
	)

	return nil
}

// DisableClient выключает подписку в панели.
func (c *RemnaClient) DisableClient(userUUID string) error {
	if err := c.changeUserState(userUUID, "disable"); err != nil {
		return err
	}

	c.logger.Info(
		"пользователь успешно выключен",
		zap.String("userUUID", userUUID),
	)

	return nil
}

// GetAllUsers получение подробных данных о каждом пользователи в панели.
func (c *RemnaClient) GetAllUsers(ctx context.Context) ([]User, error) {
	defer c.logDuration("GetAllUsers")()

	const pageSize = 100

	var allUsers []User

	for start := 0; ; start += pageSize {
		url := fmt.Sprintf(
			"%s/api/users?start=%d&size=%d&%s",
			c.cfg.PanelURL,
			start,
			pageSize,
			c.cfg.SecretURLToken,
		)

		resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		body, err := c.readBody(resp)
		c.closeBody(resp)

		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, c.wrapErr(
				fmt.Errorf("%w: %d", ErrBadStatusCode, resp.StatusCode),
				"ошибка получения всех пользователей",
				"all_users",
				url,
			)
		}

		var userResponse UsersResponse

		if err := c.parseJSON(body, &userResponse); err != nil {
			return nil, err
		}

		allUsers = append(allUsers, userResponse.Response.Users...)
		if userResponse.Response.Total > 0 && len(allUsers) >= userResponse.Response.Total {
			break
		}

		if len(userResponse.Response.Users) < pageSize {
			break
		}
	}

	c.logger.Info(
		"список пользователей RemnaWave успешно получен",
		zap.Int("users_total", len(allUsers)),
	)

	return allUsers, nil
}

// GetUserInfo - возвращает информацию.
func (c *RemnaClient) GetUserInfo(uuid string) (GetUserInfoResponse, error) {
	defer c.logDuration("GetUserInfo")()

	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.PanelURL, uuid, c.cfg.SecretURLToken)

	resp, err := c.doRequest(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return GetUserInfoResponse{}, err
	}

	defer c.closeBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var userInfo GetUserInfoResponse

		body, err := c.readBody(resp)
		if err != nil {
			return GetUserInfoResponse{}, err
		}

		if err := c.parseJSON(body, &userInfo); err != nil {
			return GetUserInfoResponse{}, err
		}

		return userInfo, nil

	case http.StatusNotFound:
		return GetUserInfoResponse{}, ErrNotFound

	case http.StatusBadRequest:
		return GetUserInfoResponse{}, ErrBadRequestUUID

	case http.StatusInternalServerError:
		return GetUserInfoResponse{}, ErrInternalServerError

	default:
		return GetUserInfoResponse{}, fmt.Errorf("%w: %d", ErrUndefined, resp.StatusCode)
	}
}

// GetUUIDByUsername - метод нахождения пользователя через username.
func (c *RemnaClient) GetUUIDByUsername(ctx context.Context, username string) (string, error) {
	defer c.logDuration("GetUUIDByUsername")()

	// Формируем URL для запроса информации о пользователе по username
	// /api/users/by-username/{username}
	url := fmt.Sprintf(
		"%s/api/users/by-username/%s?%s",
		c.cfg.PanelURL,
		username,
		c.cfg.SecretURLToken,
	)

	var userData GetUUIDByUsernameResponse

	// Выполняем HTTP запрос и парсим ответ через вспомогательный метод
	if err := c.doRequestAndParse(
		ctx,
		http.MethodGet,
		url,
		nil,
		&userData,
		username,
	); err != nil {
		return "", err
	}

	// Панель больше не отдаёт uuid — идентификатор теперь числовой id.
	// 0 значит, что пользователь не найден или пришёл пустой ответ.
	if userData.Response.ID == 0 || userData.Response.Username == "" {
		c.logger.Error(
			"received empty id or username in response",
			zap.Int("response_id", userData.Response.ID),
			zap.String("response_username", userData.Response.Username),
		)

		return "", ErrUUIDorUsernameIsNill
	}

	c.logger.Info(
		"получен id пользователя",
		zap.String("username", username),
		zap.Int("id", userData.Response.ID),
	)

	// Возвращаем id строкой: все URL панели теперь строятся с числовым id.
	return strconv.Itoa(userData.Response.ID), nil
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
	url := fmt.Sprintf("%s/api/users/%s?%s", c.cfg.PanelURL, UUID, c.cfg.SecretURLToken)

	resp, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	respBody, _ := c.readBody(resp)

	return c.handleDelete(resp, respBody, username, UUID)
}
