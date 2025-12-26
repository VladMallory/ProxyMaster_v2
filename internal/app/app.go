package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/infrastructure/telegram"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Application interface {
	Run()
}

// App зависимости приложения
type app struct {
	remnawaveClient domain.RemnawaveClient
	telegramClient  *telegram.Client
}

func New() (Application, error) {
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
		fmt.Println("ошибка инициализации бота")
	}

	// запускаем бота
	telegramClient := telegram.NewClient(bot)

	// регистрируем команды
	telegramClient.RegisterCommand(&telegram.StartCommand{})

	return &app{
		remnawaveClient: remnawaveClient,
		telegramClient:  telegramClient,
	}, nil
}

func (a *app) Run() {

	// ===telegram bot===
	a.telegramClient.Run()
}
