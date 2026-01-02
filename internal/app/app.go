// Package app тут собирается и запускается приложение
package app

import (
	"fmt"

	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/domain/telegrambot"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/service"
	"ProxyMaster_v2/pkg/logger"

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
		return nil, fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// ===logger===
	// Инициализируем главный логгер.
	logger, err := logger.New(cfg.LoggerLevel)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации логгера: %w", err)
	}

	// Создаем logger для remnawave.
	remnawaveLogger := logger.Named("remnawave")
	subscriptionLogger := logger.Named("subscription")

	// ===remnawave===
	remnawaveClient := remnawave.NewRemnaClient(cfg, remnawaveLogger)

	// ===services===
	subService := service.NewSubscriptionService(remnawaveClient, subscriptionLogger)

	// ===telegram bot===
	// инициализация
	botAPI, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации бота: %w", err)
	}

	// запускаем бота
	telegramClient := telegram.NewClient(botAPI)

	// регистрируем команды из бизнес-логики (domain/bot)
	kbBuilder := telegram.NewKeyboardBuilder()
	startCmd := telegrambot.NewStartCommand(kbBuilder, cfg.TelegramSupport, remnawaveClient)
	telegramClient.RegisterCommand(startCmd)

	// Регистрируем обработчик кнопок
	callbackHandler := telegrambot.NewCallbackHandler(subService, cfg.TelegramSupport, remnawaveClient)
	telegramClient.SetCallbackHandler(callbackHandler.Handle)

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
