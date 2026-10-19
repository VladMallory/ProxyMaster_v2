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
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/closer"
	platform "github.com/VladMallory/ProxyMaster_v2/internal/platform/postgres"
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	devicetg "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/inbound/telegram"
	devicepay "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/payment"
	postgresremnawave "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/postgres"
	remnawavedevice "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device/adapter/outbound/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/adapter/outbound/remnawave"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"gopkg.in/telebot.v4"
)

type app struct {
	bot       *telebot.Bot
	db        *sql.DB
	deviceSvc *device.Service
}

func main() {
	myApp, err := newApp()
	if err != nil {
		log.Fatalln(err)
	}

	defer closer.HelpClose(myApp.db, &err)

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

	if cerr := platform.Migrate(ctx, db); cerr != nil {
		_ = db.Close()

		return app{}, cerr
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
		logger,
		adminNotifier,
	)
	telegramNotifier := telegramhandler.NewNotifier(bot)
	usersUseCase := userscase.NewUserUseCase(remnawaveAdapter, cfg.DeviceLimit)

	subContributor, userProvider := setupSubscriptions(cfg, usersUseCase)

	deviceRepo := postgresremnawave.New(db)
	devicePanel := remnawavedevice.New(remnawavePlatform, cfg.RemnawaveAPIKey)
	deviceSvc := device.New(deviceRepo, devicePanel, logger)

	paySvc, payContributor := setupPayment(cfg, usersUseCase, telegramNotifier)

	deviceStarter := devicepay.New(paySvc, deviceSvc, telegramNotifier, cfg.ExtraDevicePriceRub)
	deviceContributor := devicetg.NewContributor(
		devicetg.NewHandler(deviceSvc, deviceStarter, logger),
	)

	registry := &platformtg.Registry{}
	registry.Register(subContributor)
	registry.Register(deviceContributor)
	registry.Register(payContributor)

	startHandler := platformtg.NewStartHandler(bot, registry, userProvider, cfg.TrialDays)
	platformtg.Setup(bot, registry, startHandler)
	platformtg.SetupCommands(bot)

	return app{
		bot:       bot,
		db:        db,
		deviceSvc: deviceSvc,
	}, nil
}

func (a app) run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runReconcile(ctx, a.deviceSvc)

	a.bot.Start()
}

func runReconcile(ctx context.Context, svc *device.Service) {
	t := time.NewTicker(time.Minute)
	since := time.Now().UTC()
	for {
		select {
		case <-ctx.Done():
			t.Stop()

			return
		case now := <-t.C:
			_ = svc.Reconcile(ctx, since)
			since = now
		}
	}
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

// setupPayment собирает фичу payment: сервис оплаты + Contributor для реестра.
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
