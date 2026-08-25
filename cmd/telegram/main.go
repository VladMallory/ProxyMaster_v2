package main

import (
	"log"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
)

func main() {
	app, err := new()
	if err != nil {
		log.Fatalln(err)
	}

	if err = app.run(); err != nil {
		log.Fatalln(err)
	}
}

type app struct {
	telegramHandler telegram.Handler
}

func new() (app, error) {
	cfg, err := config.Load()
	if err != nil {
		return app{}, err
	}

	remnawaveClient := remnawave.NewRemnawaveClient(
		cfg.RemnawaveBaseURL,
		cfg.RemnawaveToken,
		cfg.RemnawaveAPIKey,
	)

	usersUseCase := userscase.NewUserUseCase(remnawaveClient, cfg.DeviceLimit)

	tgBot, err := telegram.NewHandler(
		usersUseCase,
		cfg.TelegramToken,
		cfg.TelegramSupport,
		cfg.TrialDays,
	)
	if err != nil {
		return app{}, err
	}

	return app{
		telegramHandler: *tgBot,
	}, nil
}

func (a app) run() error {
	a.telegramHandler.Start()

	return nil
}
