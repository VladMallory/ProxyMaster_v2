package main

import (
	"context"
	"log"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	paymenttg "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/platega"
	paymentnotif "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/telegram"
	paymentservice "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	userstg "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/userscase"
	"gopkg.in/telebot.v4"
)

type app struct {
	bot *telebot.Bot
}

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatalln(err)
	}

	app.run()
}

func newApp() (app, error) {
	cfg := config.Load()

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return app{}, err
	}

	// Реестр кнопок главного меню общая третья сущность,
	// не принадлежащая ни users, ни payment.
	registry := platformtg.NewRegistry()

	remnawaveClient := remnawave.NewRemnawaveClient(
		cfg.RemnawaveBaseURL,
		cfg.RemnawaveToken,
		cfg.RemnawaveAPIKey,
	)
	usersUseCase := userscase.NewUserUseCase(remnawaveClient, cfg.DeviceLimit)

	usersHandler := userstg.NewHandler(
		bot,
		usersUseCase,
		cfg.TelegramSupport,
		cfg.TrialDays,
		registry,
	)
	usersHandler.RegisterRoutes()

	plategaClient := platega.NewClient(cfg.PlategaMerchantID, cfg.PlategaSecret)
	gateway := platega.NewGateway(
		plategaClient,
		cfg.PaymentReturnURL,
		cfg.PaymentFailedURL,
		cfg.PaymentCurrency,
	)

	// observer 1: продлевает подписку после оплаты
	subscriptionExtender := userscase.NewSubscriptionExtender(usersUseCase)

	// observer 2: пишет пользователю в Telegram после оплаты
	paymentNotifier := paymentnotif.NewNotifier(bot)

	paymentSvc := paymentservice.NewPaymentService(
		context.Background(),
		gateway,
		subscriptionExtender,
		paymentNotifier,
	)
	// paymentHandler сам регистрирует свою кнопку в реестре
	paymentHandler := paymenttg.NewHandler(bot, paymentSvc, registry)
	paymentHandler.RegisterRoutes()

	// Общий fallback регистрируется ПОСЛЕДНИМ, после всех будущих контекстов.
	err = platformtg.RegisterFallback(bot)
	if err != nil {
		return app{}, err
	}

	usersHandler.SetupCommands()

	return app{
		bot: bot,
	}, nil
}

func (a app) run() {
	a.bot.Start()
}
