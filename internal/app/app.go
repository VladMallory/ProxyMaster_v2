// Package app тут собирается и запускается приложение
package app

import (
	"fmt"

	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/pkg/logger"
)

// Application главный интерфейс приложения
type Application interface {
	Run()
}

// App зависимости приложения
type app struct {
	remnawaveClient domain.RemnawaveClient
	// plategaClient   *platega.Client
	telegramClient *telegram.Client
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
	loggerClient, err := logger.New(cfg.LoggerLevel)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации логгера: %w", err)
	}

	// Создаем logger для remnawave
	remnawaveLogger := loggerClient.Named("remnawave")
	// Для сервиса с подписками
	// subscriptionLogger := loggerClient.Named("subscription")
	// Для платежной системы
	// telegramLogger := loggerClient.Named("telegram")
	// plategaLogger := loggerClient.Named("platega")

	// ===remnawave===
	remnawaveClient := remnawave.NewRemnaClient(cfg, remnawaveLogger)

	// ===DB===
	// db, err := database.Connect(cfg.DatabaseURL)
	// if err != nil {
	// 	return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	// }

	// repository
	// userRepo := database.NewUserStorage(db)

	// ===services===
	// subService := service.NewSubscriptionService(remnawaveClient, userRepo, subscriptionLogger)

	// ===telegram bot===
	telegramClient, err := telegram.NewTelegramClient(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}
	// Инициализируем единый обработчик логики бота

	return &app{
		remnawaveClient: remnawaveClient,
		// plategaClient:   plategaClient,
		telegramClient: telegramClient,
	}, nil
}

// Run запуск приложения
func (a *app) Run() {
	// Запускаем Telegram бота в горутине
	go a.telegramClient.Start()

	// Чтобы программа не завершалась
	select {}
}
