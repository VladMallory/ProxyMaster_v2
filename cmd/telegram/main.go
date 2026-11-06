package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/distribution"
	disttg "github.com/VladMallory/ProxyMaster_v2/internal/distribution/adapter/inbound/telegram"
	distremnawave "github.com/VladMallory/ProxyMaster_v2/internal/distribution/adapter/outbound/remnawave"
	disttelegram "github.com/VladMallory/ProxyMaster_v2/internal/distribution/adapter/outbound/telegram"
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
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"gopkg.in/telebot.v4"
)

type app struct {
	bot        *telebot.Bot
	db         *sql.DB
	stopWorker context.CancelFunc
	workerDone chan struct{}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	myApp, err := newApp()
	if err != nil {
		log.Println(err)
	}

	defer func() {
		if err := myApp.db.Close(); err != nil {
			log.Println("ошибка закрытия базы!!!", err)
		}
	}()

	myApp.run(ctx)
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

	distRecipients := distremnawave.NewUsers(remnawavePlatform, cfg.RemnawaveAPIKey, logger)
	distSender := disttelegram.NewSender(bot)
	distService := distribution.NewService(distRecipients, distSender, logger)
	distHandler := disttg.NewHandler(
		distService,
		distSender,
		remnawave.ParseAdminID(cfg.TelegramAdminID),
		logger,
	)

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

	// deadcode yookassa: временная кнопка отключения автоплатежа.
	autopayContributor, autopayButton := newAutopayButton(ctx, db)
	// end deadcode yookassa

	paySvc, payContributor := setupPayment(
		ctx,
		cfg,
		db,
		usersUseCase,
		telegramNotifier,
		// deadcode yookassa: временная кнопка отключения автоплатежа.
		autopayButton,
		// end dead code
	)
	subContributor, userProvider := setupSubscriptions(cfg, usersUseCase)

	telegramHandler := telegramdevice.NewHandlerTelegram(
		deviceUseCase,
		paySvc,
		telegramNotifier,
		cfg.ExtraDevicePriceRub,
	)

	// === CONTRIBUTOR ===
	deviceContributor := telegramdevice.NewContributor(telegramHandler)

	registry := &platformtg.Registry{}
	registry.Register(subContributor)
	registry.Register(deviceContributor)
	registry.Register(payContributor)

	// deadcode yookassa: регистрация временной кнопки отключения автоплатежа.
	if autopayContributor != nil {
		registry.Register(autopayContributor)
	}
	// end deadcode yookassa

	startHandler := platformtg.NewStartHandler(
		bot,
		registry,
		userProvider,
		cfg.TrialDays,
		cfg.TrafficLimit,
	)
	platformtg.Setup(bot, registry, startHandler)
	bot.Handle(disttg.Command, distHandler.Handle)
	platformtg.SetupCommands(bot)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		deviceUseCase.RunExpiryWorker(workerCtx, device.DefaultExpiryInterval)
	}()

	return app{
		bot:        bot,
		db:         db,
		stopWorker: stopWorker,
		workerDone: workerDone,
	}, nil
}

func (a app) run(ctx context.Context) {
	go a.bot.Start()

	<-ctx.Done()
	a.bot.Stop()
	a.stopWorker()
	<-a.workerDone
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

// paymentServiceFactory собирает провайдера оплат по конфигу.
var paymentServiceFactory = func(
	_ context.Context,
	cfg config.Config,
	_ *sql.DB,
) paymentsvc.PaymentService {
	return newDefaultPaymentService(cfg)
}

// buildTariffs собирает тарифы из PRICE_PER_MONTH и сроков в днях.
// Цена пропорциональна сроку: 30 дней стоят ровно PRICE_PER_MONTH.
func buildTariffs(cfg config.Config) []paymentdomain.Tariff {
	if cfg.PricePerMonth <= 0 {
		log.Fatalf("PRICE_PER_MONTH должен быть больше нуля")
	}

	days := []int{
		cfg.InitialPriceDays1,
		cfg.InitialPriceDays2,
		cfg.InitialPriceDays3,
		cfg.InitialPriceDays4,
	}

	tariffs := make([]paymentdomain.Tariff, 0, len(days))

	for _, d := range days {
		if d <= 0 {
			continue
		}

		tariffs = append(tariffs, paymentdomain.Tariff{
			Term:     subdomain.Days(d),
			PriceRub: priceForDays(cfg.PricePerMonth, d),
		})
	}

	if len(tariffs) == 0 {
		log.Fatalf("не задан ни один INITIAL_PRICE_DAYS1..4")
	}

	return tariffs
}

// priceForDays цена за срок без скидки: месяц это 30 дней.
func priceForDays(pricePerMonth, days int) int {
	return (pricePerMonth*days + 15) / 30
}

func setupPayment(
	ctx context.Context,
	cfg config.Config,
	db *sql.DB,
	subs userscase.UserUseCase,
	notifier *telegramhandler.Notifier,
	autopayButton telegramhandler.AutopayButton, // deadcode yookassa: временная кнопка отключения автоплатежа.
) (*paymentsvc.Service, platformtg.MenuContributor) {
	paymentService := paymentServiceFactory(ctx, cfg, db)

	tariffs := buildTariffs(cfg)

	paySvc := paymentsvc.NewPayment(paymentService, notifier, tariffs)
	// deadcode yookassa: при удалении вернуть NewHandler(paySvc, notifier, subs).
	handler := telegramhandler.NewHandler(paySvc, notifier, subs, autopayButton)
	// end deadcode

	return paySvc, telegramhandler.NewPaymentContributor(handler)
}

func newDefaultPaymentService(cfg config.Config) paymentsvc.PaymentService {
	switch cfg.PaymentProvider {
	case "platega":
		return platega.NewClient(
			cfg.PlategaMerchantID,
			cfg.PlategaSecret,
			cfg.PlategaReturnURL,
			cfg.PlategaReturnURL,
		)
	case "fake":
		return fake.NewClient(5 * time.Second)
	default:
		log.Fatalf(
			"Неизвестный провайдер %q: нужен platega или fake в .env",
			cfg.PaymentProvider,
		)
	}

	return nil
}
