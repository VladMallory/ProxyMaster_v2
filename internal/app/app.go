// Package app тут собирается и запускается приложение
package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/service"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Application главный интерфейс приложения
type Application interface {
	Run()
}

// App зависимости приложения
type app struct {
	remnawaveClient domain.RemnawaveClient
	telegramClient  *telegram.Client
}

// New собирает приложение
func New() (Application, error) {
	// ===конфиг .env===
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	// ===remnawave===
	remnawaveClient := remnawave.NewRemnaClient(cfg)

	// ===services===
	// Создает сервис подписок и передает его remnawave клиенту
	subscriptionService := service.NewSubscriptionService(remnawaveClient)

	// ===telegram bot===
	// инициализация
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации бота: %w", err)
	}

	// запускаем бота
	telegramClient := telegram.NewClient(bot, subscriptionService)

	// регистрируем команды
	telegramClient.RegisterCommand(&telegram.StartCommand{})

	return &app{
		remnawaveClient: remnawaveClient,
		telegramClient:  telegramClient,
	}, nil
}

// Run запуск приложения
func (a *app) Run() {
	// ===telegram bot===
	a.telegramClient.Run()
}
