package telegram

import (
	"context"
	"html"
	"log/slog"
	"strconv"

	"gopkg.in/telebot.v4"
)

// handleStart обрабатывает /start: получает подписку и показывает главное меню.
func (h *Handler) handleStart(c telebot.Context) error {
	text, menu, err := h.startMenu(c)
	if err != nil {
		return c.Send(err.Error())
	}

	return c.Send(text, menu, telebot.ModeHTML)
}

// startMenu собирает текст приветствия и клавиатуру стартового меню.
func (h *Handler) startMenu(c telebot.Context) (string, *telebot.ReplyMarkup, error) {
	user := c.Sender()

	users, err := h.useCase.GetOrCreateSub(
		context.Background(),
		strconv.FormatInt(user.ID, 10),
		h.trialDays,
	)
	if err != nil {
		return "", nil, err
	}

	text, err := renderStart(startViewModel{
		Name:       html.EscapeString(user.FirstName),
		ExpireDate: formatExpireDate(users.ExpireAt),
		Device:     users.Device,
	})
	if err != nil {
		return "", nil, err
	}

	slog.Info("подписка получена",
		"users", users.Name,
	)

	return text, h.keys.Start(users), nil
}
