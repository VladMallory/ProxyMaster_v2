package main

import (
	"context"
	"log"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/platega"
	paymenttg "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram"
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
		log.Fatalln(err)
	}

	remnawaveClient := remnawave.NewRemnawaveClient(
		cfg.RemnawaveBaseURL,
		cfg.RemnawaveToken,
		cfg.RemnawaveAPIKey,
	)
	usersUseCase := userscase.NewUserUseCase(remnawaveClient, cfg.DeviceLimit)

	usersHandler := userstg.NewHandler(bot, usersUseCase, cfg.TelegramSupport, cfg.TrialDays)
	usersHandler.RegisterRoutes()

	gateway := platega.New()

	// observer №1: продлевает подписку после оплаты
	subscriptionExtender := userscase.NewSubscriptionExtender(usersUseCase)

	// observer №2: пишет пользователю в Telegram после оплаты
	paymentNotifier := paymentnotif.NewNotifier(bot)

	paymentSvc := paymentservice.NewPaymentService(
		context.Background(),
		gateway,
		subscriptionExtender,
		paymentNotifier,
	)
	paymentHandler := paymenttg.NewHandler(bot, paymentSvc)
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
