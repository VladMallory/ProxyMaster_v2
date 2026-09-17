package telegram

import (
	"context"
	"html"
	"log/slog"
	"strconv"
	"time"

	"gopkg.in/telebot.v4"
)

type UserProvider interface {
	GetOrCreateSub(ctx context.Context, username string, trialDays int) (UserView, error)
}

type UserView struct {
	Name     string
	URL      string
	Device   int
	ExpireAt time.Time
}

type StartHandler struct {
	bot       *telebot.Bot
	registry  *Registry
	users     UserProvider
	trialDays int
}

func NewStartHandler(
	bot *telebot.Bot,
	registry *Registry,
	users UserProvider,
	trialDays int,
) *StartHandler {
	return &StartHandler{
		bot:       bot,
		registry:  registry,
		users:     users,
		trialDays: trialDays,
	}
}

// HandleStart обработчик /start: получает подписку, рендерит шаблон, отдаёт меню из Registry.
func (h *StartHandler) HandleStart(c telebot.Context) error {
	text, menu, err := h.buildStart(c)
	if err != nil {
		return c.Send(err.Error())
	}

	return c.Send(text, menu, telebot.ModeHTML)
}

// buildStart общая логика для HandleStart и HandleBack.
func (h *StartHandler) buildStart(c telebot.Context) (string, *telebot.ReplyMarkup, error) {
	user := c.Sender()

	u, err := h.users.GetOrCreateSub(
		context.Background(),
		strconv.FormatInt(user.ID, 10),
		h.trialDays,
	)
	if err != nil {
		return "", nil, err
	}

	text, err := renderStart(startViewModel{
		Name:       html.EscapeString(user.FirstName),
		ExpireDate: formatExpireDate(u.ExpireAt),
		Device:     u.Device,
	})
	if err != nil {
		return "", nil, err
	}

	slog.Info("подписка получена", "user", u.Name)

	// собираем меню из всех вкладчиков URL динамический через MenuContent
	menu := h.registry.BuildStartMenu(MenuContent{SubURL: u.URL})

	return text, menu, nil
}

// HandleBack редактирует текущее сообщение обратно в главное меню.
func (h *StartHandler) HandleBack(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}
	text, menu, err := h.buildStart(c)
	if err != nil {
		return err
	}

	return c.Edit(text, menu, telebot.ModeHTML)
}

// RenderMainMenuForTimeout строит меню где нету telebot.Context.
// TODO: переделать под универсальное меню чтобы не дублировать.

// func (h *StartHandler) RenderMainMenuForTimeout(
// 	ctx context.Context,
// 	userID string,
// ) (string, *telebot.ReplyMarkup, error) {
// 	u, err := h.users.GetOrCreateSub(ctx, userID, h.trialDays)
// 	if err != nil {
// 		return "", nil, err
// 	}
//
// 	text, err := renderStart(startViewModel{
// 		Name:       html.EscapeString(u.Name),
// 		ExpireDate: formatExpireDate(u.ExpireAt),
// 		Device:     u.Device,
// 	})
// 	if err != nil {
// 		return "", nil, err
// 	}
//
// 	menu := h.registry.BuildStartMenu(MenuContent{SubURL: u.URL})
//
// 	return text, menu, nil
// }
