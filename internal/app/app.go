// Package app тут собирается и запускается приложение
package app

import (
	"fmt"

	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/database"
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
	// plategaClient   *platega.Client
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

	// Создаем logger для remnawave
	remnawaveLogger := logger.Named("remnawave")
	// Для сервиса с подписками
	subscriptionLogger := logger.Named("subscription")
	// Для платежной системы
	// plategaLogger := logger.Named("platega")

	// ===remnawave===
	remnawaveClient := remnawave.NewRemnaClient(cfg, remnawaveLogger)

	// ===DB===
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	// repository
	userRepo := database.NewUserStorage(db)

	// ===services===
	subService := service.NewSubscriptionService(remnawaveClient, userRepo, subscriptionLogger)

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

	// plategaClient := platega.NewClient(cfg.PlategaAPIKey, plategaLogger)
	// data, _ := plategaClient.CreateTransaction(context.Background(), platega.SBPQR, 100, platega.RUB, "test", "test")
	// fmt.Println(data)

	// SetDeviceID - устанавливает DeviceID для пользователя
	deviceType := uint8(5)
	err = remnawaveClient.SetDevices("asdasdad", &deviceType)
	fmt.Println(err)

	return &app{
		remnawaveClient: remnawaveClient,
		telegramClient:  telegramClient,
		// plategaClient:   plategaClient,
	}, nil
}

// Run запуск приложения
func (a *app) Run() {
	// ===telegram bot===
	a.telegramClient.Run()
}
