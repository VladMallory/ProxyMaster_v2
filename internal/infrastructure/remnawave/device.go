package remnawave

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/VladMallory/ProxyMaster_v2/internal/models"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

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

	url := fmt.Sprintf("%s/api/users?%s", c.cfg.RemnaPanelURL, c.cfg.RemnaSecretURLToken)

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
		logger.Field{Key: "username", Value: username},
		logger.Field{Key: "devices", Value: *devices},
		logger.Field{Key: "status_code", Value: response.StatusCode},
	)

	return nil
}

func (c *RemnaClient) DeleteDeviceHWID(ctx context.Context, username string) error {
	defer c.logDuration("DeleteDeviceHWID")

	// Получаем UUID
	UUID, err := c.GetUUIDByUsername(ctx, username)
	if err = c.wrapErr(err, "ошибка получения UUID", username); err != nil {
		return err
	}

	// URL для удаления устройств
	url := fmt.Sprintf("%s/api/hwid/devices/delete-all?%s",
		c.cfg.RemnaPanelURL,
		c.cfg.RemnaSecretURLToken,
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
