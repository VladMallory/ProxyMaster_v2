package telegram

import (
	"context"
	"html"
	"log/slog"
	"strconv"
	"time"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	"gopkg.in/telebot.v4"
)

type Handler struct {
	useCase    userscase.UserUseCase
	bot        *telebot.Bot
	trialDays  int
	supportURL string
}

func NewHandler(useCase userscase.UserUseCase, token, supportURL string) (*Handler, error) {
	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return &Handler{}, err
	}

	tgBot := &Handler{
		useCase:    useCase,
		bot:        b,
		supportURL: supportURL,
	}

	tgBot.registerRoutes()

	menuButton := []telebot.Command{
		{
			Text:        "start",
			Description: "Вызывать главное меню",
		},
	}

	if err := b.SetCommands(menuButton); err != nil {
		slog.Warn("err:", "error", err)
	}

	return tgBot, nil
}

func (h *Handler) Start() {
	h.bot.Start()
}

func (h *Handler) Stop() {
	h.bot.Stop()
}

// RegisterRoutes связывает команды telebotgram с методами хендлера.
func (h *Handler) registerRoutes() {
	h.bot.Handle("/start", h.handleStart)

	h.bot.Handle(telebot.OnText, h.handleUnknownCommand)
}

func (h *Handler) handleStart(c telebot.Context) error {
	user := c.Sender()

	users, err := h.useCase.GetOrCreateSub(
		context.Background(),
		strconv.FormatInt(user.ID, 10),
		h.trialDays,
	)
	if err != nil {
		return c.Send(err.Error())
	}

	menu := h.keysStart(users)

	text, err := renderStart(startViewModel{
		Name:       html.EscapeString(user.FirstName),
		ExpireDate: formatExpireDate(users.ExpireAt),
		Device:     users.Device,
	})
	if err != nil {
		return err
	}

	err = c.Send(text, menu, telebot.ModeHTML)
	if err != nil {
		return err
	}

	slog.Info("подписка получена",
		"users", users.Name,
	)

	return nil
}

func (h Handler) keysStart(users subdomain.User) *telebot.ReplyMarkup {
	// Клавиатуры
	menu := &telebot.ReplyMarkup{}

	btnURL := menu.URL("🚀 Подключиться", users.URL)
	btnSupport := menu.URL("🛟 Поддержка", h.supportURL)

	menu.Inline(
		menu.Row(btnURL),
		menu.Row(btnSupport),
	)

	return menu
}

func (h *Handler) handleUnknownCommand(c telebot.Context) error {
	return c.Send("Неизвестнная команда, введите /start")
}
