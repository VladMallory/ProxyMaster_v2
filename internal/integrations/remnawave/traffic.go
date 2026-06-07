package remnawave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

func (c *RemnaClient) BetterResetTraffic(ctx context.Context, username string) error {
	defer c.logDuration("BetterResetTraffic")()

	UUID, err := c.GetUUIDByUsername(ctx, username)
	switch {
	case err == nil:
		c.logger.Info("UUID получен успешно")
	case errors.Is(err, ErrNotFound):
		return c.wrapErr(err, "User not found", username)
	case errors.Is(err, ErrInternalServerError):
		return c.wrapErr(err, "Internal server error", username)
	default:
		return c.wrapErr(err, "Failed to get UUID", username)
	}

	url := fmt.Sprintf(
		"%s/api/users/%s/actions/reset-traffic?%s",
		c.cfg.PanelURL,
		UUID,
		c.cfg.SecretURLToken,
	)

	resp, err := c.doRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	defer c.closeBody(resp)

	body, err := c.readBody(resp)
	if err != nil {
		return err
	}

	if _, err := c.handleUpdate(resp, body); err != nil {
		return err
	}

	return nil
}

// ===НЕ ИСПОЛЬЗУЕТСЯ===

// SetTraffic устанавливает новое значение трафика.
func (c *RemnaClient) SetTraffic(username string, gb uint64) error {
	defer c.logDuration("SetTraffic")()

	// Преобразуем в гиги за шаги
	const bytesInGB uint64 = 1024 * 1024 * 1024

	trafficLimitBytes := gb * bytesInGB

	// Формируем запрос на изменение лимита трафика
	userData := &UpdateUserRequest{
		Username:          &username,
		TrafficLimitBytes: &trafficLimitBytes,
	}

	url := fmt.Sprintf(
		"%s/api/users?%s",
		c.cfg.PanelURL,
		c.cfg.SecretURLToken,
	)

	resp, err := c.doRequest(context.Background(), http.MethodPatch, url, userData)
	if err != nil {
		return nil
	}
	defer c.closeBody(resp)

	body, err := c.readBody(resp)
	if err != nil {
		return err
	}

	if _, err := c.handleUpdate(resp, body); err != nil {
		return err
	}

	return nil
}
