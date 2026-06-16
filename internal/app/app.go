// Package app тут собирается и запускается приложение
//
// nolint: forbidigo
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/delivery/transport/telegram"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	"github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/device"
	subscriptionhttp "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/transport/http"
	"github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/users"
	"github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/users/reminders"
	remindertg "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/users/reminders/handler/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/payments/platega"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/payments/youkassa"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/db"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/http"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/logger"
)

// Application главный интерфейс приложения.
type Application interface {
	Run()
}

type ReminderRunnerUser interface {
	RunDay(ctx context.Context)
}

// BotRunner для запуска бота.
type BotRunner interface {
	Start()
}

type ReminderRunnerDevice interface {
	RunBillingLoop(ctx context.Context)
}

// App зависимости приложения.
type app struct {
	deviceBilling ReminderRunnerDevice
	reminder      ReminderRunnerUser
	bot           BotRunner
	server        *http.Server
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
	yookassaLogger := loggerClient.Named("yookassa")
	plategaLogger := loggerClient.Named("platega")
	databaseLogger := loggerClient.Named("database")
	serverLogger := loggerClient.Named("server")

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

	dbConn, err := db.Connect(cfg.DatabaseURL, databaseLogger)
	if err != nil {
		return nil, err
	}

	// repository
	userRepo := db.NewUserStorage(dbConn, databaseLogger)

	// ===services===
	// users subscription
	subscriptionUserSvc := users.NewSubscriptionService(
		remnawaveClient,
		userRepo,
		subscriptionLogger,
	)

	// device
	deviceService := device.NewDeviceService(
		remnawaveClient,
		userRepo,
		userRepo,
		subscriptionLogger,
		cfg.DeviceLimit,
		cfg.MaxDeviceLimit,
		cfg.ExtraDevicePrice,
	)

	deviceBilling := device.NewDeviceBillingService(
		remnawaveClient,
		userRepo,
		subscriptionLogger,
		cfg.DeviceLimit,
		cfg.ExtraDevicePrice,
	)

	// ===Платежная система===
	var paymentGateway billingSvc.PaymentGateway

	switch cfg.PaymentProvider {
	case "yookassa":
		paymentGateway = youkassa.NewClient(
			cfg.YouKassaShopID,
			cfg.YouKassaSecretKey,
			cfg.YouKassaReturnURL,
			yookassaLogger,
		)
	case "platega":
		paymentGateway = platega.NewClient(
			cfg.PlategaMerchantID,
			cfg.PlategaAPIKey,
			cfg.PlategaReturnURL,
			plategaLogger,
		)
	default:
		return nil, errors.New("в .env нужно указать платежную систему")
	}

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
		subscriptionUserSvc,
		deviceService,
		paymentGateway,
		userRepo,
		telegramAdminID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	// ===REMINDER===
	reminderSvc := reminders.New(
		remnawaveClient,
		remindertg.NewSender(telegramClient),
		subscriptionReminderServiceLogger,
	)

	// ===server===
	server := http.NewServer(serverLogger, "web/site/static")

	subHTTPHandler := subscriptionhttp.NewHandler(subscriptionUserSvc, subscriptionLogger)
	subscriptionhttp.RegisterRoutes(server.Mux(), subHTTPHandler)

	return &app{
		deviceBilling: deviceBilling,
		reminder:      reminderSvc,
		bot:           telegramClient,
		server:        server,
	}, nil
}

// Run запуск приложения.
func (a *app) Run() {
	ctx := context.Background()

	// Запускаем Telegram бота в горутине
	go a.bot.Start()

	// Сервис о напоминии об оплате
	go a.reminder.RunDay(ctx)
	// Проверка доп. устройств
	go a.deviceBilling.RunBillingLoop(ctx)

	// Сервер
	go func() {
		if err := a.server.Run(); err != nil {
			log.Fatal("http server: ", err)
		}
	}()

	fmt.Println("программа запущена")

	// Чтобы программа не завершалась
	select {}
}
