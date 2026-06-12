// Package telegram содержит бизнес-логику обработки команд и отображения экранов Telegram-бота.
// Взаимодействие с Telegram API делегируется в internal/integrations/telegram.
//
//nolint:cyclop,funlen
package telegram

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/delivery/transport/telegram/viewtype"
	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	tgintegration "github.com/VladMallory/ProxyMaster_v2/internal/integrations/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/db"

	"go.uber.org/zap"
)

// Client это обертка, которая содержит бизнес-зависимости и интеграционный клиент Telegram API.
type Client struct {
	api                 tgintegration.BotAPI
	logger              *zap.Logger
	remnawaveClient     remnawave.RemnawaveClient
	subscriptionService SubscriptionService
	deviceService       DeviceService
	billingSvc          *billingSvc.Service
	userRepo            *db.UserStorage
	adminID             int64
	cfg                 *config.Config
}

// NewTelegramClient создает нового клиента для Telegram.
func NewTelegramClient(
	token string,
	cfg *config.Config,
	logger *zap.Logger,
	remnawaveClient remnawave.RemnawaveClient,
	subscriptionService SubscriptionService,
	deviceService DeviceService,
	paymentGateway billingSvc.PaymentGateway,
	userRepo *db.UserStorage,
	adminID int64,
) (*Client, error) {
	socks5 := tgintegration.SOCKS5Config{
		Host:     cfg.SOCKS5Host,
		Port:     cfg.SOCKS5Port,
		Username: cfg.SOCKS5Username,
		Password: cfg.SOCKS5Password,
	}

	api, err := tgintegration.New(token, socks5)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	billingClient := billingSvc.New(paymentGateway)

	return &Client{
		api:                 api,
		cfg:                 cfg,
		logger:              logger.Named("telegram"),
		remnawaveClient:     remnawaveClient,
		subscriptionService: subscriptionService,
		deviceService:       deviceService,
		billingSvc:          billingClient,
		userRepo:            userRepo,
		adminID:             adminID,
	}, nil
}

// Start запускает бесконечный цикл получения обновлений от Telegram.
// Делегирует polling в integrations/telegram, передавая callback с бизнес-логикой.
func (c *Client) Start() {
	// регистрирует кнопки три полоски слева снизу в телеграмме
	if err := c.api.SetupCommandsAndMenu(); err != nil {
		c.logger.Error("ошибка в добавлении кнопок",
			zap.Error(err),
		)
	}

	c.api.Start(func(msg tgintegration.Message) {
		if msg.IsCallback {
			if err := ProcessCallback(
				c,
				msg.ChatID,
				msg.MessageID,
				msg.Data,
				msg.FirstName,
				c.remnawaveClient,
				c.subscriptionService,
				c.deviceService,
				c.billingSvc,
				c.userRepo,
				c.cfg,
			); err != nil {
				c.logger.Error("Ошибка обработки callback", zap.Error(err))
			}

			return
		}

		ProcessCommand(
			c,
			msg.ChatID,
			msg.Text,
			msg.FirstName,
			msg.TelegramUsername,
			c.remnawaveClient,
			c.userRepo,
			c.logger,
			c.adminID,
		)
	})
}

// ShowView отправляет или редактирует сообщение с inline-клавиатурой.
func (c *Client) ShowView(
	chatID int64,
	messageID int,
	viewType viewtype.ViewType,
	data string,
) error {
	var text string

	var keyboard tgintegration.InlineKeyboard

	switch viewType {
	case viewtype.ViewTypeDownloadApp:
		text, keyboard = c.handleDownloadAppView()
	case viewtype.ViewTypeIosRegion:
		text, keyboard = c.handleIosRegionView()
	case viewtype.ViewTypeTopUp:
		text, keyboard = c.handleTopUpView()

	case viewtype.ViewTypeCheckPayment:
		text, keyboard = c.handleCheckPaymentView(data)
	case viewtype.ViewTypeProfile:
		text, keyboard = c.handleProfileView(data)
	case viewtype.ViewTypeDeviceLimits:
		text, keyboard = c.handleDeviceLimitsView(data)
	case viewtype.ViewTypeTrafficLimits:
		text, keyboard = c.handleTrafficLimitsView()
	case viewtype.ViewTypeConnect:
		text, keyboard = c.handleConnectView(data)
	case viewtype.ViewTypeSubscriptionResult:
		text, keyboard = c.handleSubscriptionResultView(data)
	case viewtype.ViewTypeMain:
		text, keyboard = c.handleMainView(data, chatID)
	case viewtype.ViewTypeServiceInfo:
		text, keyboard = c.handleServiceInfoView()
	case viewtype.ViewTypePrivacyPolicy:
		text, keyboard = c.handlePrivacyPolicyView()
	case viewtype.ViewTypeUserAgreement:
		text, keyboard = c.handleUserAgreementView()
	default:
		return fmt.Errorf("%w: %s", domain.ErrUserNotFound, viewType)
	}

	if messageID > 0 {
		return c.api.EditMessage(chatID, messageID, text, keyboard)
	}

	return c.api.SendMessage(chatID, text, keyboard)
}

// SendMessage отправляет текстовое сообщение пользователю с главной клавиатурой.
func (c *Client) SendMessage(chatID int64, text string) error {
	return c.api.SendMessage(chatID, text, c.mainKeyboard(chatID))
}

func (c *Client) handleCheckPaymentView(data string) (string, tgintegration.InlineKeyboard) {
	parts := strings.Split(data, "|")
	url := parts[0]

	text := "Ссылка на оплату сформирована. Я автоматически проверю платеж после оплаты."
	keyboard := c.checkPaymentKeyboard(url)

	return text, keyboard
}

func (c *Client) handleSubscriptionResultView(data string) (string, tgintegration.InlineKeyboard) {
	return data, handleBackView()
}

func (c *Client) handleDownloadAppView() (string, tgintegration.InlineKeyboard) {
	return "Какое у вас устройство?", c.downloadAppKeyboard()
}

func (c *Client) handleIosRegionView() (string, tgintegration.InlineKeyboard) {
	text := "Выберите какой у вас регион App Store.\n\n" +
		"Если не знаете, попробуйте сначала RU, если выдаст ошибку, то 'другие регионы'"

	return text, c.iosRegionKeyboard()
}

func (c *Client) handleTopUpView() (string, tgintegration.InlineKeyboard) {
	return "💰 Выберите сумму для пополнения баланса:", c.topUpKeyboard()
}

func (c *Client) handleConnectView(data string) (string, tgintegration.InlineKeyboard) {
	var text string
	if data == "" {
		text = "Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку."
	} else {
		text = "Ваша ссылка для подключения:\n" + data
	}

	return text, c.connectKeyboard()
}

const welcomeText = "📱 <b>Управление устройствами и сброс трафика</b>\n\n" +
	"Здесь вы можете увеличить количество дополнительных устройств и сбросить намотанные гигабайты."

func (c *Client) handleDeviceLimitsView(data string) (string, tgintegration.InlineKeyboard) {
	text := data
	if text == "" {
		text = welcomeText
	}

	return text, c.deviceLimitsKeyboard()
}

func (c *Client) handleMainView(data string, userID int64) (string, tgintegration.InlineKeyboard) {
	var text string
	if data != "" {
		text = data
	} else {
		text = "🌟 Добро пожаловать."
	}

	return text, c.mainKeyboard(userID)
}

func (c *Client) handleProfileView(data string) (string, tgintegration.InlineKeyboard) {
	parts := strings.SplitN(data, "|", 4)
	userID := ""
	balance := "0"
	extraDevices := "0"
	nextPayment := "—"

	if len(parts) > 0 {
		userID = parts[0]
	}

	if len(parts) > 1 {
		balance = parts[1]
	}

	if len(parts) > 2 {
		extraDevices = parts[2]
	}

	if len(parts) > 3 && parts[3] != "" {
		nextPayment = parts[3]
	}

	extraDevicesInt, err := strconv.Atoi(extraDevices)
	if err != nil {
		extraDevicesInt = 0
	}

	extraCharge := max(
		(extraDevicesInt-c.cfg.DeviceLimit)*50, 0)

	text := fmt.Sprintf(
		"ID пользователя: %s\nБаланс: %s ₽\nЛимит устройств: %d\nДоп. списание: %d ₽/мес\nНужно оплатить до: %s",
		userID,
		balance,
		extraDevicesInt,
		extraCharge,
		nextPayment,
	)
	keyboard := c.profileKeyboard()

	return text, keyboard
}

func (c *Client) handleTrafficLimitsView() (string, tgintegration.InlineKeyboard) {
	text := "Сбросить использованный трафик (обнулить намотанные гигабайты)?\n\nСтоимость: 50₽"

	return text, c.trafficLimitsKeyboard()
}

func (c *Client) handleServiceInfoView() (string, tgintegration.InlineKeyboard) {
	return "Выберите раздел:", c.serviceInfoKeyboard()
}

func (c *Client) handlePrivacyPolicyView() (string, tgintegration.InlineKeyboard) {
	content, err := os.ReadFile("assets/police.txt")
	if err != nil {
		c.logger.Error("ошибка чтения файла",
			zap.ByteString("file", content),
			zap.Error(err),
		)

		return "не удалось загрузить файл политики конфиденциальности", handleBackView()
	}

	return string(content), handleBackView()
}

func (c *Client) handleUserAgreementView() (string, tgintegration.InlineKeyboard) {
	content, err := os.ReadFile("assets/user_agreement.txt")
	if err != nil {
		c.logger.Error("ошибка чтения файла",
			zap.String("file", "assets/user_agreement.txt"),
			zap.Error(err),
		)

		return "не удалось загрузить файл пользовательского соглашения", handleBackView()
	}

	return string(content), handleBackView()
}

func (c *Client) mainKeyboard(userID int64) tgintegration.InlineKeyboard {
	username := strconv.FormatInt(userID, 10)
	url, err := c.subscriptionService.GetURLSubscription(username)

	var connectButton tgintegration.ButtonData

	if err != nil || url == "" {
		connectButton = tgintegration.NewButton(
			"📱 Подключить (Happ)",
			"btn_connect_error",
		)
	} else {
		connectButton = tgintegration.NewURLButton(
			"📱 Подключить (Happ)",
			url,
		)
	}

	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton("📱 Скачать приложение", "download_app"),
		},
		{
			connectButton,
		},
		{
			tgintegration.NewButton("💰 Пополнить баланс", "btn_balance"),
		},
		{
			tgintegration.NewButton("👤 Увеличение лимитов", "btn_unlimits"),
			tgintegration.NewURLButton("🛟 Поддержка", c.cfg.TelegramSupport),
		},
		{
			tgintegration.NewButton("🧾 Информация о сервисе", "btn_info"),
		},
	}
}
