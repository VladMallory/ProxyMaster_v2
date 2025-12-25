package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/infrastructure/telegram"
	"context"
	"errors"
	"log"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// зависимости приложения
type App struct {
	remnawaveClient domain.RemnawaveClient
	telegramHandler *telegram.Handler
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
			slog.Warn("Пользователь уже существует", "error", err)
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

	// запускаем бота
	telegranHandler := telegram.NewHandler(bot)

	return &App{
		remnawaveClient: remnawaveClient,
		telegramHandler: telegranHandler,
	}, nil
}

func (a *App) Run() {
	ctx := context.Background()

	// ===remnawave===
	a.remnawaveClient.GetServiceInfo(ctx, "")

	// ===telegram bot===
	a.telegramHandler.Run()
}
