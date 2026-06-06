package remnawave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"go.uber.org/zap"
)

type HWIDDevice struct {
	HWID        string     `json:"hwid"`
	UserUUID    string     `json:"userUuid"`
	Platform    *string    `json:"platform"`
	OSVersion   *string    `json:"osVersion"`
	DeviceModel *string    `json:"deviceModel"`
	UserAgent   *string    `json:"userAgent"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

type GetUserDevicesResponse struct {
	Response struct {
		Total   int          `json:"total"`
		Devices []HWIDDevice `json:"devices"`
	} `json:"response"`
}

func (c *RemnaClient) GetUserDevice(
	ctx context.Context,
	username string,
) ([]HWIDDevice, error) {
	defer c.logDuration("GetUserDevice")()

	userUUID, err := c.GetUUIDByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"%s/api/hwid/devices/%s?%s",
		c.cfg.PanelURL,
		userUUID,
		c.cfg.SecretURLToken,
	)

	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer c.closeBody(resp)

	body, err := c.readBody(resp)
	if err != nil {
		return nil, err
	}

	var response GetUserDevicesResponse
	if err := c.parseJSON(body, &response); err != nil {
		return nil, err
	}

	c.logger.Info(
		"устройства пользователя успешно получены",
		zap.String("username", username),
		zap.String("user_uuid", userUUID),
		zap.Int("devices_count", len(response.Response.Devices)),
	)

	return response.Response.Devices, nil
}

// SetDevices устанавилвает кол-во устройств пользователя.
func (c *RemnaClient) SetDevices(ctx context.Context, username string, devices *uint8) error {
	if devices == nil {
		return ErrDevicesNotSet
	}

	defer c.logDuration("SetDevices")()

	// Отправляем только то что нужно изменить, без идентификаторов в теле
	userData := &models.UpdateUserRequest{
		Username:        &username,
		HwidDeviceLimit: devices,
	}

	url := fmt.Sprintf("%s/api/users?%s", c.cfg.PanelURL, c.cfg.SecretURLToken)

	response, err := c.doRequest(ctx, http.MethodPatch, url, userData)
	if err != nil {
		return err
	}

	defer c.closeBody(response)

	body, err := c.readBody(response)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToReadBody, err)
	}

	_, err = c.handleUpdate(response, body)
	c.wrapErr(err, "handleUpdate", username, url)

	// Метод успешно отработал
	c.logger.Info(
		"успешное измение количества устройств",
		zap.String("username", username),
		zap.Uint8("devices", *devices),
		zap.Int("status_code", response.StatusCode),
	)

	return nil
}

func (c *RemnaClient) DeleteDeviceHWID(ctx context.Context, username string) error {
	defer c.logDuration("DeleteDeviceHWID")()

	// Получаем UUID
	UUID, err := c.GetUUIDByUsername(ctx, username)
	if err = c.wrapErr(err, "ошибка получения UUID", username); err != nil {
		return err
	}

	// URL для удаления устройств
	url := fmt.Sprintf("%s/api/hwid/devices/delete-all?%s",
		c.cfg.PanelURL,
		c.cfg.SecretURLToken,
	)

	// JSON запрос с UUID пользователя
	requestBody := map[string]string{
		"userUuid": UUID,
	}

	// Делаем POST запрос
	response, err := c.doRequest(ctx, http.MethodPost, url, requestBody)
	if err = c.wrapErr(err, "ошибка POST запроса", username); err != nil {
		return err
	}

	defer c.closeBody(response)

	// Читаем тело ответа
	body, err := c.readBody(response)
	if err != nil {
		c.wrapErr(err, "readBody", username, url)

		return errors.New("ошибка чтения ответа")
	}

	_, err = c.handleCreate(response, body)
	if err != nil {
		c.wrapErr(err, "handleCreate", username, url)

		return errors.New("ошибка обработки ответа")
	}

	return nil
}
