package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/infrastructure/telegram"
	"errors"
	"log"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// зависимости приложения
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

	if err = remnawaveClient.CreateUser("asd11213", 30); err != nil {
		if errors.Is(err, remnawave.ErrBadRequestCreate) {
			slog.Warn("Пользователь нажал /start, в панели уже есть подписка", "TEXT", err)
		} else {
			return nil, err
		}
	}

	// ===telegram bot===
	// инициализация
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal("ошибка в инициализации бота", err)
	}

	// data, err := remnawaveClient.GetUserInfo("f4d45e65-8a85-4731-9858-1a3545fc68a8")
	// fmt.Println(data)
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
