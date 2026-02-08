// Package telegram отвечает за взаимодействие с Telegram API
//
//nolint:cyclop,funlen
package telegram

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	domainTelegram "ProxyMaster_v2/internal/domain/telegram"
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Чтобы линтер не жаловался на дублирование кода.
const parseModeHTML = "HTML"

// Client это обертка над стандартной библиотекой tgbotapi.
// Он хранит в себе подключение к API и умеет отправлять сообщения.
type Client struct {
	api                 *tgbotapi.BotAPI
	logger              logger.Logger
	remnawaveClient     domain.RemnawaveClient
	subscriptionService domain.SubscriptionService
	cfg                 *config.Config
}

// NewTelegramClient создает нового клиента для Telegram.
// token: токен бота, который мы получили от BotFather.
func NewTelegramClient(token string, cfg *config.Config, logger logger.Logger) (*Client, error) {
	// Инициализируем библиотеку с токеном
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	return &Client{
		api:    api,
		cfg:    cfg,
		logger: logger.Named("telegram"),
	}, nil
}

// Start это "сердце" бота. Метод запускает бесконечный цикл,
// который слушает обновления от Telegram (сообщения, нажатия кнопок)
// и передает их в бизнес-логику (domain).
func (c *Client) Start(
	remnawaveClient domain.RemnawaveClient,
	subscriptionService domain.SubscriptionService,
	paymentGateway domain.PaymentGateway,
	userRepo *database.UserStorage,
	adminID int64,
) {
	c.remnawaveClient = remnawaveClient
	c.subscriptionService = subscriptionService
	// Настраиваем конфигурацию получения обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60 // Ждем 60 секунд новых сообщений (long polling)

	// Получаем канал, в который будут падать обновления
	updates, err := c.api.GetUpdatesChan(u)
	if err != nil {
		c.logger.Error("Ошибка получения обновлений", logger.Field{Key: "error", Value: err})

		return
	}

	// Читаем обновления из канала в бесконечном цикле
	for update := range updates {
		// 1. Обработка нажатий на инлайн-кнопки (CallbackQuery)
		if update.CallbackQuery != nil {
			// Передаем нажатие в слой бизнес-логики, включая ID сообщения для редактирования
			err := domainTelegram.ProcessCallback(
				c,
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				update.CallbackQuery.Data,
				update.CallbackQuery.From.FirstName,
				remnawaveClient,
				subscriptionService,
				paymentGateway,
				userRepo,
				c.cfg,
			)
			if err != nil {
				c.logger.Error("Ошибка обработки callback", logger.Field{Key: "error", Value: err})
			}

			// Обязательно отвечаем Telegram, что мы приняли нажатие (иначе у юзера будет крутиться "часики")
			cd := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := c.api.AnswerCallbackQuery(cd); err != nil {
				c.logger.Error("Ошибка ответа на callback query",
					logger.Field{Key: "error", Value: err},
				)
			}

			continue
		}

		// 2. Обработка обычных текстовых сообщений
		if update.Message != nil {
			firstName := ""
			if update.Message.From != nil {
				firstName = update.Message.From.FirstName
			}

			// Передаем текст сообщения в слой бизнес-логики
			err := domainTelegram.ProcessCommand(
				c,
				update.Message.Chat.ID,
				update.Message.Text,
				firstName,
				remnawaveClient,
				// subscriptionService,
				userRepo,
				c.logger,
				adminID)
			if err != nil {
				c.logger.Error("Ошибка обработки команды", logger.Field{Key: "error", Value: err})
			}
		}
	}
}

// ShowView отправляет сообщение с нужной клавиатурой в зависимости от типа
// Если messageID > 0, то сообщение редактируется. Иначе — отправляется новое.
func (c *Client) ShowView(
	chatID int64,
	messageID int,
	viewType domain.ViewType,
	data string,
) error {
	var text string

	var keyboard tgbotapi.InlineKeyboardMarkup

	// viewType
	switch viewType {
	case domain.ViewTypeDownloadApp:
		text, keyboard = c.handleDownloadAppView()
	case domain.ViewTypeIosRegion:
		text, keyboard = c.handleIosRegionView()
	// case domain.ViewTypeTariffs:
	// 	text, keyboard = c.handleTariffsView()
	case domain.ViewTypeTopUp:
		text, keyboard = c.handleTopUpView()

	case domain.ViewTypeCheckPayment:
		text, keyboard = c.handleCheckPaymentView(data)
	case domain.ViewTypeProfile:
		text, keyboard = c.handleProfileView(data)
	case domain.ViewTypeDeviceLimits:
		text, keyboard = c.handleDeviceLimitsView()
	case domain.ViewTypeTrafficLimits:
		text, keyboard = c.handleTrafficLimitsView()
	case domain.ViewTypeConnect:
		text, keyboard = c.handleConnectView(data)
	case domain.ViewTypeSubscriptionResult:
		text, keyboard = c.handleSubscriptionResultView(data)
	case domain.ViewTypeMain:
		text, keyboard = c.handleMainView(data, chatID)
	case domain.ViewTypeServiceInfo:
		text, keyboard = c.handleServiceInfoView()
	case domain.ViewTypePrivacyPolicy:
		text, keyboard = c.handlePrivacyPolicyView()
	case domain.ViewTypeUserAgreement:
		text, keyboard = c.handleUserAgreementView()
	default:
		return fmt.Errorf("%w: %s", domain.ErrUserNotFound, viewType)
	}

	if messageID > 0 {
		// Редактируем существующее сообщение
		msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		// Добавляем форматирование parseModeHTML
		msg.ParseMode = parseModeHTML
		msg.ReplyMarkup = &keyboard

		_, err := c.api.Send(msg)
		if err != nil {
			return fmt.Errorf("ошибка редактирования сообщения: %w", err)
		}

		return nil
	}

	// Отправляем новое сообщение
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	// Добавляем форматирование parseModeHTML
	msg.ParseMode = parseModeHTML

	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки нового сообщения: %w", err)
	}

	return nil
}

// SendMessage отправляет текстовое сообщение пользователю.
// Также всегда прикрепляет главную клавиатуру, чтобы она не пропадала.
// Реализует метод интерфейса domain.MessageSender.
func (c *Client) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	// Добавляем форматирование parseModeHTML
	msg.ParseMode = parseModeHTML

	// Всегда показываем главное меню под сообщением
	msg.ReplyMarkup = c.mainKeyboard(chatID)

	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения: %w", err)
	}

	return nil
}

func (c *Client) handleCheckPaymentView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	// data format: "url|transactionID"
	parts := strings.Split(data, "|")
	url := parts[0]

	text := "Ссылка на оплату сформирована. Я автоматически проверю платеж после оплаты."
	keyboard := c.checkPaymentKeyboard(url)

	return text, keyboard
}

func (c *Client) handleSubscriptionResultView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	return data, handleBackView()
}

func (c *Client) handleDownloadAppView() (string, tgbotapi.InlineKeyboardMarkup) {
	return "Какое у вас устройство?", c.downloadAppKeyboard()
}

func (c *Client) handleIosRegionView() (string, tgbotapi.InlineKeyboardMarkup) {
	text := "Выберите какой у вас регион App Store.\n\n" +
		"Если не знаете, попробуйте сначала RU, если выдаст ошибку, то 'другие регионы'"

	return text, c.iosRegionKeyboard()
}

// func (c *Client) handleTariffsView() (string, tgbotapi.InlineKeyboardMarkup) {
// 	return "Выберите тариф подписки:", c.tariffsKeyboard()
// }

func (c *Client) handleTopUpView() (string, tgbotapi.InlineKeyboardMarkup) {
	return "💰 Выберите сумму для пополнения баланса:", c.topUpKeyboard()
}

func (c *Client) handleConnectView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	var text string
	if data == "" {
		text = "Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку."
	} else {
		text = "Ваша ссылка для подключения:\n" + data
	}

	return text, c.connectKeyboard()
}

// handleDeviceLimitsView отображает меню для управления лимитами устройств.
// Позволяет пользователю выбрать между просмотром профиля или возвратом в главное меню.
func (c *Client) handleDeviceLimitsView() (string, tgbotapi.InlineKeyboardMarkup) {
	text := "В этом меню вы можете увеличить количество дополнительных устройств и увеличить лимиты трафика."

	return text, c.deviceLimitsKeyboard()
}

func (c *Client) handleMainView(data string, userID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	var text string
	if data != "" {
		text = data
	} else {
		text = "🌟 Добро пожаловать."
	}

	return text, c.mainKeyboard(userID)
}

func (c *Client) handleProfileView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
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

	text := fmt.Sprintf(
		"ID пользователя: %s\nБаланс: %s ₽\nДоп. устройств: %d\nДоп. списание: %d ₽/мес\nНужно оплатить до: %s",
		userID,
		balance,
		extraDevicesInt,
		extraDevicesInt*50,
		nextPayment,
	)
	keyboard := c.profileKeyboard()

	return text, keyboard
}

func (c *Client) handleTrafficLimitsView() (string, tgbotapi.InlineKeyboardMarkup) {
	text := "Сбросить использованный трафик (обнулить намотанные гигабайты)?\n\nСтоимость: 50₽"

	return text, c.trafficLimitsKeyboard()
}

func (c *Client) handleServiceInfoView() (string, tgbotapi.InlineKeyboardMarkup) {
	text := "Выберите раздел:"

	return text, c.serviceInfoKeyboard()
}

// handlePrivacyPolicyView читает текст политики конфиденциальности и возвращает
// вместе с соотвествующей клавиатурой.
func (c *Client) handlePrivacyPolicyView() (string, tgbotapi.InlineKeyboardMarkup) {
	// Читаем файл
	content, err := os.ReadFile("assets/police.txt")
	if err != nil {
		c.logger.Error("ошибка чтения файла",
			logger.Field{Key: "file", Value: content},
			logger.Field{Key: "error", Value: err},
		)

		text := "не удалось загрузить файл политики конфиденциальности"

		return text, handleBackView()
	}

	// Если все хорошо, возвращаем текст и клавиатуру
	return string(content), handleBackView()
}

// handleUserAgreementView читает текст пользовательского соглашения и возвращает
// вместе с соотвествующей клавиатурой.
func (c *Client) handleUserAgreementView() (string, tgbotapi.InlineKeyboardMarkup) {
	// Читаем файл
	content, err := os.ReadFile("assets/user_agreement.txt")
	if err != nil {
		c.logger.Error("ошибка чтения файла",
			logger.Field{Key: "file", Value: "assets/user_agreement.txt"},
			logger.Field{Key: "error", Value: err},
		)

		text := "не удалось загрузить файл пользовательского соглашения"

		return text, handleBackView() // Можно использовать ту же клавиатуру
	}

	// Если все хорошо, возвращаем текст и клавиатуру
	return string(content), handleBackView()
}

// mainKeyboard создает структуру кнопок для главного меню.
// Используем Inline кнопки (прозрачные, под сообщением).
func (c *Client) mainKeyboard(userID int64) tgbotapi.InlineKeyboardMarkup {
	// Получение URL подписки
	username := strconv.FormatInt(userID, 10)
	url, err := c.subscriptionService.GetURLSubscription(username)

	var connectButton tgbotapi.InlineKeyboardButton

	if err != nil || url == "" {
		// Не получили подписку - показываем кнопку с сообщением об ошибке
		connectButton = tgbotapi.NewInlineKeyboardButtonData(
			"📱 Подключить (Happ)",
			"btn_connect_error",
		)
	} else {
		// Если URL есть, делаем кнопку с ссылкой
		connectButton = tgbotapi.NewInlineKeyboardButtonURL(
			"📱 Подключить (Happ)",
			url,
		)
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Скачать приложение", "download_app"),
		),
		tgbotapi.NewInlineKeyboardRow(
			connectButton,
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Пополнить баланс", "btn_balance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Увеличение лимитов", "btn_unlimits"),
			tgbotapi.NewInlineKeyboardButtonURL("🛟 Поддержка", c.cfg.TelegramSupport),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧾 Информация о сервисе", "btn_info"),
		),
	)
}
