package remnawave

import (
	"context"
	"fmt"
	"net/http"
)

func (c *RemnaClient) AddInternalSquad(
	ctx context.Context,
	username string,
	squadTitles []string,
) error {
	c.logDuration("AddInternalSquad")()
	url := fmt.Sprintf("%s/api/users/?%s", c.cfg.PanelURL, c.cfg.SecretURLToken)

	data := &UpdateUserRequest{
		Username:             &username,
		ActiveInternalSquads: squadTitles,
	}

	response, err := c.doRequest(ctx, http.MethodPatch, url, data)
	if err := c.wrapErr(err, "ошибка отправки запроса", username, url); err != nil {
		return err
	}

	defer c.closeBody(response)

	body, err := c.readBody(response)
	if err != nil {
		return err
	}

	_, err = c.handleUpdate(response, body)
	if err := c.wrapErr(err, "ошибка проверки status code", username, url); err != nil {
		return err
	}

	return err
}
