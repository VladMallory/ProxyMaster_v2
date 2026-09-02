package telegram

import (
	"log"

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

// NewHandler больше не создаёт бота только регистрируется в уже готовом.
func NewHandler(
	bot *telebot.Bot,
	useCase userscase.UserUseCase,
	supportURL string,
	trialDays int,
) *Handler {
	return &Handler{
		useCase:   useCase,
		bot:       bot,
		trialDays: trialDays,
		keys:      keyboard.New(supportURL),
	}
}

// RegisterRoutes регистрирует все обработчики этого контекста.
// Unique-имена кнопок с префиксом users_, чтобы не столкнуться с другими контекстами позже.
func (h *Handler) RegisterRoutes() {
	h.bot.Handle("/start", h.handleStart)

	h.bot.Handle(&telebot.Btn{Unique: "users_download"}, h.handleDownload)
	h.bot.Handle(&telebot.Btn{Unique: "users_dl_ios"}, h.handleIOS)
	h.bot.Handle(&telebot.Btn{Unique: "users_dl_android"}, h.handleAndroid)
	h.bot.Handle(&telebot.Btn{Unique: "users_dl_linux"}, h.handleLinux)
	h.bot.Handle(&telebot.Btn{Unique: "users_dl_macos"}, h.handleMacOS)
	h.bot.Handle(&telebot.Btn{Unique: "users_dl_router"}, h.handleRouter)
	h.bot.Handle(&telebot.Btn{Unique: "users_back_platforms"}, h.handleBackPlatforms)
	h.bot.Handle(&telebot.Btn{Unique: "users_back"}, h.handleBack)
}

// SetupCommands регистрирует меню команд Telegram для этого контекста.
func (h *Handler) SetupCommands() {
	err := h.bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Вызывать главное меню"},
	})
	if err != nil {
		log.Fatalln(err)
	}
}
