package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/infrastructure/telegram"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// App зависимости приложения
type App struct {
	remnawaveClient domain.RemnawaveClient
	telegramClient  *telegram.Client
}

func New() (*App, error) {
	// ===конфиг .env===
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	// ===remnawave===
	remnawaveClient := remnawave.NewRemnaClient(cfg)

	// ===telegram bot===
	// инициализация
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal("ошибка в инициализации бота", err)
	}

	// запускаем бота
	telegramClient := telegram.NewClient(bot)

	// регистрируем команды
	telegramClient.RegisterCommand(&telegram.StartCommand{})

	return &App{
		remnawaveClient: remnawaveClient,
		telegramClient:  telegramClient,
	}, nil
}

func (a *App) Run() {

	// ===telegram bot===
	a.telegramClient.Run()
}
