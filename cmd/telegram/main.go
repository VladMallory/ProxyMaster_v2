package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	telegramhandler "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/fake"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/outbound/platega"
	paymentdomain "github.com/VladMallory/ProxyMaster_v2/internal/payment/domain"
	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	platform "github.com/VladMallory/ProxyMaster_v2/internal/platform/postgres"
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	telegramdevice "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/inbound/telegram"
	devicepostgres "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/postgres"
	deviceremnawave "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"gopkg.in/telebot.v4"
)

type app struct {
	bot *telebot.Bot
	db  *sql.DB
}

func main() {
	myApp, err := newApp()
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if err := myApp.db.Close(); err != nil {
			log.Println("ошибка закрытия базы!!!", err)
		}
	}()

	myApp.run()
}

//nolint:funlen
func newApp() (app, error) {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := platform.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return app{}, err
	}

	if err = platform.Migrate(ctx, db); err != nil {
		_ = db.Close()

		return app{}, err
	}

	bot, err := newBot(cfg)
	if err != nil {
		_ = db.Close()

		return app{}, err
	}

	logger := zaplogger.New(zaplogger.Config{
		LogLevel: cfg.LoggerLevel,
		Encoding: cfg.Encoding,
	})

	remnawavePlatform := platformremnawave.New(cfg.RemnawaveBaseURL, cfg.RemnawaveToken, logger)

	adminNotifier := remnawave.NewAdminNotifier(bot, remnawave.ParseAdminID(cfg.TelegramAdminID))

	remnawaveAdapter := remnawave.NewRemnawaveClient(
		remnawavePlatform,
		cfg.RemnawaveAPIKey,
		cfg.RemnawaveSquadUUID,
		logger,
		adminNotifier,
	)
	telegramNotifier := telegramhandler.NewNotifier(bot)
	usersUseCase := userscase.NewUserUseCase(remnawaveAdapter, cfg.DeviceLimit)

	deviceRepo := devicepostgres.NewDeviceDB(db)
	devicePanel := deviceremnawave.NewPanel(remnawavePlatform, cfg.RemnawaveAPIKey)

	deviceUseCase := device.NewService(
		deviceRepo,
		devicePanel,
		cfg.DeviceLimit,
		cfg.MaxDeviceLimit,
		logger,
	)

	paySvc, payContributor := setupPayment(cfg, usersUseCase, telegramNotifier)
	subContributor, userProvider := setupSubscriptions(cfg, usersUseCase)

	telegramHandler := telegramdevice.NewHandlerTelegram(
		deviceUseCase,
		paySvc,
		telegramNotifier,
		cfg.ExtraDevicePriceRub,
	)

	go deviceUseCase.RunExpiryWorker(context.Background(), device.DefaultExpiryInterval)

	// === CONTRIBUTOR ===
	deviceContributor := telegramdevice.NewContributor(telegramHandler)

	registry := &platformtg.Registry{}
	registry.Register(subContributor)
	registry.Register(deviceContributor)
	registry.Register(payContributor)

	startHandler := platformtg.NewStartHandler(
		bot,
		registry,
		userProvider,
		cfg.TrialDays,
		cfg.TrafficLimit,
	)
	platformtg.Setup(bot, registry, startHandler)
	platformtg.SetupCommands(bot)

	return app{
		bot: bot,
		db:  db,
	}, nil
}

func (a app) run() {
	a.bot.Start()
}

func newBot(cfg config.Config) (*telebot.Bot, error) {
	return telebot.NewBot(telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
}

func setupSubscriptions(
	cfg config.Config,
	usersUseCase userscase.UserUseCase,
) (platformtg.MenuContributor, platformtg.UserProvider) {
	handler := telegram.NewHandler(usersUseCase, cfg.TelegramSupport, cfg.TrialDays)
	contributor := telegram.NewSubscriptionContributor(cfg.TelegramSupport, handler)
	userProvider := telegram.NewPlatformUserAdapter(usersUseCase)

	return contributor, userProvider
}

func setupPayment(
	cfg config.Config,
	subs userscase.UserUseCase,
	notifier *telegramhandler.Notifier,
) (*paymentsvc.Service, platformtg.MenuContributor) {
	var paymentService paymentsvc.PaymentService

	switch cfg.PaymentProvider {
	case "platega":
		paymentService = platega.NewClient(
			cfg.PlategaMerchantID,
			cfg.PlategaSecret,
			cfg.PlategaReturnURL,
			cfg.PlategaReturnURL,
		)
	case "fake":
		paymentService = fake.NewClient(5 * time.Second)
	default:
		log.Fatalf(
			"Неизвестный провайдер %q: нужен platega или fake в .env",
			cfg.PaymentProvider,
		)
	}

	tariffs := []paymentdomain.Tariff{
		{Months: 1, PriceRub: cfg.PricePerMonth},
		{Months: 2, PriceRub: cfg.PricePerMonth * 2},
		{Months: 3, PriceRub: cfg.PricePerMonth * 3},
		{Months: 5, PriceRub: cfg.PricePerMonth * 5},
	}

	paySvc := paymentsvc.NewPayment(paymentService, notifier, tariffs)
	handler := telegramhandler.NewHandler(paySvc, notifier, subs)

	return paySvc, telegramhandler.NewPaymentContributor(handler)
}
