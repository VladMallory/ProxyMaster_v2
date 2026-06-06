// Package app тут собирается и запускается приложение
//
// nolint: forbidigo
package app

import (
	"context"
	"fmt"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	"github.com/VladMallory/ProxyMaster_v2/internal/delivery/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/infrastructure/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/payments/platega"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
	"github.com/VladMallory/ProxyMaster_v2/internal/service"
)

// Application главный интерфейс приложения.
type Application interface {
	Run()
}

type ReminderRunner interface {
	RunDay(ctx context.Context)
}

// BotRunner для запуска бота.
type BotRunner interface {
	Start()
}

// App зависимости приложения.
type app struct {
	bot      BotRunner
	reminder ReminderRunner
}

// New собирает приложение.
//
//nolint:funlen
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

	// Создаем под каждый сервис логгер.
	remnawaveLogger := loggerClient.Named("remnawave")
	subscriptionReminderServiceLogger := loggerClient.Named("subscription_reminder")
	subscriptionLogger := loggerClient.Named("subscription")
	plategaLogger := loggerClient.Named("platega")
	databaseLogger := loggerClient.Named("database")

	// Берем глобальный cfg и передаем только нужные поля
	remnaCfg := remnawave.RemnaConfig{
		PanelURL:       cfg.RemnaPanelURL,
		SecretURLToken: cfg.RemnaSecretURLToken,
		APIKey:         cfg.RemnaKey,
		SquadUUID:      cfg.RemnaSquadUUID,
		TrafficLimitGB: cfg.TrafficLimit,
		DeviceLimit:    cfg.DeviceLimit,
	}

	remnawaveClient := remnawave.NewRemnaClient(remnaCfg, remnawaveLogger)

	db, err := database.Connect(cfg.DatabaseURL, databaseLogger)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	// repository
	userRepo := database.NewUserStorage(db, databaseLogger)

	// ===services===
	subService := service.NewSubscriptionService(
		remnawaveClient,
		userRepo,
		subscriptionLogger,
		cfg.DeviceLimit,
	)

	// ===platega===
	plategaClient := platega.NewClient(
		cfg.PlategaMerchantID,
		cfg.PlategaAPIKey,
		cfg.PlategaReturnURL,
		plategaLogger,
	)

	// Парсим TelegramAdminID с проверкой ошибки
	telegramAdminID, err := strconv.ParseInt(cfg.TelegramAdminID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("неверный TelegramAdminID: %w", err)
	}

	// ===telegram bot===
	telegramClient, err := telegram.NewTelegramClient(
		cfg.TelegramToken,
		cfg,
		loggerClient,
		remnawaveClient,
		subService,
		plategaClient,
		userRepo,
		telegramAdminID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	subscriptionReminderService := service.NewSubscriptionReminderService(
		remnawaveClient,
		telegramClient,
		subscriptionReminderServiceLogger,
	)

	return &app{
		bot:      telegramClient,
		reminder: subscriptionReminderService,
	}, nil
}

// Run запуск приложения.
func (a *app) Run() {
	ctx := context.Background()

	// Запускаем Telegram бота в горутине
	go a.bot.Start()

	// Сервис о напоминии об оплате
	go a.reminder.RunDay(ctx)

	println("программа запущена")

	// Чтобы программа не завершалась
	select {}
}
