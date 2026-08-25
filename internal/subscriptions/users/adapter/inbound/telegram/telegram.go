package telegram

import (
	"log/slog"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram/keyboard"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	"gopkg.in/telebot.v4"
)

type Handler struct {
	useCase   userscase.UserUseCase
	bot       *telebot.Bot
	trialDays int
	keys      *keyboard.Keyboard
}

func NewHandler(
	useCase userscase.UserUseCase,
	token, supportURL string,
	trialDays int,
) (*Handler, error) {
	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return &Handler{}, err
	}

	tgBot := &Handler{
		useCase:   useCase,
		bot:       b,
		trialDays: trialDays,
		keys:      keyboard.New(supportURL),
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

// RegisterRoutes регистрирует обработчики.
func (h *Handler) registerRoutes() {
	h.bot.Handle("/start", h.handleStart)

	h.bot.Handle(&telebot.Btn{Unique: "btn_download"}, h.handleDownload)
	h.bot.Handle(&telebot.Btn{Unique: "dl_ios"}, h.handleIOS)
	h.bot.Handle(&telebot.Btn{Unique: "dl_android"}, h.handleAndroid)
	h.bot.Handle(&telebot.Btn{Unique: "dl_linux"}, h.handleLinux)
	h.bot.Handle(&telebot.Btn{Unique: "dl_macos"}, h.handleMacOS)
	h.bot.Handle(&telebot.Btn{Unique: "dl_router"}, h.handleRouter)

	h.bot.Handle(&telebot.Btn{Unique: "btn_back_platforms"}, h.handleBackPlatforms)
	h.bot.Handle(&telebot.Btn{Unique: "btn_back"}, h.handleBack)

	h.bot.Handle(telebot.OnText, h.handleUnknownCommand)
}

// === СКАЧАТЬ ПРИЛОЖЕНИЕ ===

func (h *Handler) handleDownload(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit("Выберите платформу:", h.keys.DownloadApps(), telebot.ModeHTML)
}

func (h *Handler) handleIOS(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit("Выберите App Store:", h.keys.IOS(), telebot.ModeHTML)
}

func (h *Handler) handleAndroid(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit(
		"Если у вас есть Google Play, то жмите на первую кнопку, если у вас не установлен Google Play, то на вторую:",
		h.keys.Android(),
		telebot.ModeHTML,
	)
}

func (h *Handler) handleLinux(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit("Выберите дистрибитив:", h.keys.Linux(), telebot.ModeHTML)
}

func (h *Handler) handleBackPlatforms(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit("Выберите платформу:", h.keys.DownloadApps(), telebot.ModeHTML)
}

func (h *Handler) handleMacOS(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit("Выберите вариант установки", h.keys.Macos(), telebot.ModeHTML)
}

const router string = "Если ваш роутер на Keenetic OS или OpenWRT, то можно будет поставить " +
	"прям на роутер, но это не прям очень легко, нужно будет чуть-чуть повозиться"

func (h *Handler) handleRouter(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	return c.Edit(
		router,
		h.keys.Router(),
		telebot.ModeHTML,
	)
}

func (h *Handler) handleUnknownCommand(c telebot.Context) error {
	return c.Send("Неизвестнная команда, введите /start")
}

func (h *Handler) handleBack(c telebot.Context) error {
	err := c.Respond()
	if err != nil {
		return err
	}

	text, menu, err := h.startMenu(c)
	if err != nil {
		return err
	}

	// Возвращаем главное меню, редактируя текущее сообщение
	return c.Edit(text, menu, telebot.ModeHTML)
}
