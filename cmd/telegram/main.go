package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	autoconnecthapp "github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/inbound/http/happ"
	autoconnectincy "github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/inbound/http/incy"
	autoconnecttg "github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/inbound/telegram"
	autoconnectcrypto "github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/outbound/happcrypto"
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
	userscase "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/service"
	supportbtn "github.com/VladMallory/ProxyMaster_v2/internal/support/adapter/inbound/telegram"
	zaplogger "github.com/VladMallory/ProxyMaster_v2/pkg/zap"
	"gopkg.in/telebot.v4"
)

type app struct {
	bot        *telebot.Bot
	db         *sql.DB
	web        *http.Server
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

	autoConnect := autoconnect.New(cfg.AutoConnectSub, cfg.LinkSecret)
	webServer := newWebServer(cfg.WebAddr, autoConnect, cfg.ProviderName)

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

	paySvc, payContributor := setupPayment(cfg, usersUseCase, telegramNotifier)
	subContributor, userProvider := setupSubscriptions(cfg, usersUseCase)

	telegramHandler := telegramdevice.NewHandlerTelegram(
		deviceUseCase,
		paySvc,
		telegramNotifier,
		cfg.ExtraDevicePriceRub,
	)

	// === CONTRIBUTOR ===
	deviceContributor := telegramdevice.NewContributor(telegramHandler)
	autoConnectContributor := autoconnecttg.NewContributor(autoConnect)

	registry := &platformtg.Registry{}
	registry.Register(subContributor)
	registry.Register(autoConnectContributor)
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
	bot.Handle(disttg.Command, distHandler.Handle)
	platformtg.SetupCommands(bot)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		deviceUseCase.RunExpiryWorker(workerCtx, device.DefaultExpiryInterval)
	}()

	// === SUPPORTBTN ===
	supportBtn := supportbtn.NewSupportBtn(cfg.TelegramSupport)
	registry.Register(supportBtn)

	return app{
		bot:        bot,
		db:         db,
		web:        webServer,
		stopWorker: stopWorker,
		workerDone: workerDone,
	}, nil
}

func (a app) run(ctx context.Context) {
	go a.bot.Start()

	if a.web != nil {
		go func() {
			if err := a.web.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("web: %v", err)
			}
		}()
	}

	<-ctx.Done()

	if a.web != nil {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := a.web.Shutdown(shutdownCtx)
		cancel()

		if err != nil {
			log.Printf("web shutdown: %v", err)
		}
	}

	a.bot.Stop()
	a.stopWorker()
	<-a.workerDone
}

// newWebServer порт для автоимпорта подписок.
func newWebServer(addr string, links *autoconnect.Service, providerName string) *http.Server {
	if addr == "" || !links.CheckEnabled() {
		return nil
	}

	happCrypto := autoconnectcrypto.NewClient("https://crypto.happ.su/api-v2.php")

	mux := http.NewServeMux()
	autoconnecthapp.NewHandler(links, happCrypto).Register(mux, "/happ")
	autoconnectincy.NewHandler(links, providerName).Register(mux, "/incy")

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
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
	handler := telegram.NewHandler(usersUseCase, cfg.TrialDays)
	contributor := telegram.NewSubscriptionContributor(handler)
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
