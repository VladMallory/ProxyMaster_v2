package main

import (
	"log"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
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

	usersHandler := telegram.NewHandler(bot, usersUseCase, cfg.TelegramSupport, cfg.TrialDays)
	usersHandler.RegisterRoutes()

	// Общий fallback регистрируется ПОСЛЕДНИМ, после всех будущих контекстов.
	platformtg.RegisterFallback(bot)

	usersHandler.SetupCommands()

	return app{
		bot: bot,
	}, nil
}

func (a app) run() {
	a.bot.Start()
}
