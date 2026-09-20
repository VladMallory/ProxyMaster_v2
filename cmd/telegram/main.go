package main

import (
	"log"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	telegramhandler "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/platega"
	paymentdomain "github.com/VladMallory/ProxyMaster_v2/internal/payment/domain"
	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
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
	myApp, err := newApp()
	if err != nil {
		log.Fatalln(err)
	}

	myApp.run()
}

func newApp() (app, error) {
	cfg := config.Load()

	bot, err := newBot(cfg)
	if err != nil {
		return app{}, err
	}

	remnawavePlatform := platformremnawave.New(cfg.RemnawaveBaseURL, cfg.RemnawaveToken)

	remnawaveAdapter := remnawave.NewRemnawaveClient(
		remnawavePlatform,
		cfg.RemnawaveAPIKey,
	)
	telegramNotifier := telegramhandler.NewNotifier(bot)
	usersUseCase := userscase.NewUserUseCase(remnawaveAdapter, cfg.DeviceLimit)

	subContributor, userProvider := setupSubscriptions(cfg, usersUseCase)

	payContributor := setupPayment(cfg, usersUseCase, telegramNotifier)

	registry := &platformtg.Registry{}
	registry.Register(subContributor)
	registry.Register(payContributor)

	startHandler := platformtg.NewStartHandler(bot, registry, userProvider, cfg.TrialDays)
	platformtg.Setup(bot, registry, startHandler)
	platformtg.SetupCommands(bot)

	return app{bot: bot}, nil
}

func newBot(cfg config.Config) (*telebot.Bot, error) {
	return telebot.NewBot(telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
}

// setupSubscriptions собирает всё для фичи subscriptions.
func setupSubscriptions(
	cfg config.Config,
	usersUseCase userscase.UserUseCase,
) (platformtg.MenuContributor, platformtg.UserProvider) {
	handler := telegram.NewHandler(usersUseCase, cfg.TelegramSupport, cfg.TrialDays)
	contributor := telegram.NewSubscriptionContributor(cfg.TelegramSupport, handler)
	userProvider := telegram.NewPlatformUserAdapter(usersUseCase)

	return contributor, userProvider
}

// setupPayment собирает всё для фичи payment и отдаёт её Contributor.
func setupPayment(
	cfg config.Config,
	extender paymentsvc.SubscriptionExtender,
	notifier *telegramhandler.Notifier,
) platformtg.MenuContributor {
	plategaClient := platega.NewClient(
		cfg.PlategaBaseURL,
		cfg.PlategaMerchantID,
		cfg.PlategaSecret,
		cfg.PlategaReturnURL,
		cfg.PlategaReturnURL,
	)

	tariffs := []paymentdomain.Tariff{
		{Months: 1, PriceRub: cfg.PricePerMonth},
		{Months: 2, PriceRub: cfg.PricePerMonth * 2},
		{Months: 3, PriceRub: cfg.PricePerMonth * 3},
		{Months: 5, PriceRub: cfg.PricePerMonth * 5},
	}

	paySvc := paymentsvc.NewPayment(plategaClient, extender, notifier, tariffs)
	handler := telegramhandler.NewHandler(paySvc, notifier)

	return telegramhandler.NewPaymentContributor(handler)
}

func (a app) run() {
	a.bot.Start()
}
