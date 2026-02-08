// Package app тут собирается и запускается приложение
package app

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/database"
	restapi "ProxyMaster_v2/internal/delivery/restAPI"
	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"ProxyMaster_v2/internal/payments/youkassa"
	"ProxyMaster_v2/internal/service"
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"log"
	"strconv"
)

// Application главный интерфейс приложения.
type Application interface {
	Run()
}

// App зависимости приложения.
type app struct {
	remnawaveClient     domain.RemnawaveClient
	paymentGateway      domain.PaymentGateway
	subscriptionService domain.SubscriptionService
	telegramClient      *telegram.Client
	userRepo            *database.UserStorage
	restAPI             domain.ServerAPI
	telegramAdminID     int64
}

// New собирает приложение.
func New() (Application, error) {
	// ===конфиг .env===
	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// ===logger===
	// Инициализируем главный логгер.
	loggerClient, err := logger.NewZap(cfg.LoggerLevel)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации логгера: %w", err)
	}

	// Создаем под каждый сервис логгер.
	remnawaveLogger := loggerClient.Named("remnawave")
	subscriptionLogger := loggerClient.Named("subscription")
	youkassaLogger := loggerClient.Named("youkassa")
	restAPILogger := loggerClient.Named("restAPI")
	databaseLogger := loggerClient.Named("database")

	remnawaveClient := remnawave.NewRemnaClient(cfg, remnawaveLogger)

	db, err := database.Connect(cfg.DatabaseURL, databaseLogger)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	// repository
	//
	userRepo := database.NewUserStorage(db, databaseLogger)

	// ===services===
	subService := service.NewSubscriptionService(remnawaveClient, userRepo, subscriptionLogger)

	// ===youkassa===
	youkassaClient := youkassa.NewClient(
		cfg.YouKassaShopID,
		cfg.YouKassaSecretKey,
		cfg.YouKassaReturnURL,
		youkassaLogger,
	)

	// ===telegram bot===
	telegramClient, err := telegram.NewTelegramClient(cfg.TelegramToken, cfg, loggerClient)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	// ===restAPI===
	handler := restapi.NewHandler(remnawaveClient, subService)
	restAPI := restapi.New(handler, restAPILogger)

	// Парсим TelegramAdminID с проверкой ошибки
	telegramAdminID, err := strconv.ParseInt(cfg.TelegramAdminID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("неверный TelegramAdminID: %w", err)
	}

	return &app{
		remnawaveClient:     remnawaveClient,
		paymentGateway:      youkassaClient,
		subscriptionService: subService,
		telegramClient:      telegramClient,
		userRepo:            userRepo,
		restAPI:             restAPI,
		telegramAdminID:     telegramAdminID,
	}, nil
}

// Run запуск приложения.
func (a *app) Run() {
	// Запускаем Telegram бота в горутине
	go a.telegramClient.Start(
		a.remnawaveClient,
		a.subscriptionService,
		a.paymentGateway,
		a.userRepo,
		a.telegramAdminID,
	)

	// запускаем restAPI
	go func() {
		if err := a.restAPI.Serve(":8080"); err != nil {
			log.Fatal(err)
		}
	}()

	println("программа запущена")

	// Чтобы программа не завершалась
	select {}
}
