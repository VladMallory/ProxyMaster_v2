// Package app тут собирается и запускается приложение
//
// nolint: forbidigo
package app

import (
	"fmt"
	"log"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/database"
	restapi "github.com/VladMallory/ProxyMaster_v2/internal/delivery/restAPI"
	"github.com/VladMallory/ProxyMaster_v2/internal/delivery/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/infrastructure/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/payments/youkassa"
	"github.com/VladMallory/ProxyMaster_v2/internal/service"
	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

// Application главный интерфейс приложения.
type Application interface {
	Run()
}

// App зависимости приложения.
type app struct {
	telegramClient *telegram.Client
	restAPI        domain.ServerAPI
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

	// ===youkassa===
	youkassaClient := youkassa.NewClient(
		cfg.YouKassaShopID,
		cfg.YouKassaSecretKey,
		cfg.YouKassaReturnURL,
		youkassaLogger,
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
		youkassaClient,
		userRepo,
		telegramAdminID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	// ===restAPI===
	handler := restapi.NewHandler(remnawaveClient, subService)
	restAPI := restapi.New(handler, restAPILogger)

	return &app{
		telegramClient: telegramClient,
		restAPI:        restAPI,
	}, nil
}

// Run запуск приложения.
func (a *app) Run() {
	// Запускаем Telegram бота в горутине
	go a.telegramClient.Start()

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
