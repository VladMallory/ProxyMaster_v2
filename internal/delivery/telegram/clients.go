// Package telegram отвечает за взаимодействие с Telegram API
//
//nolint:cyclop
package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	domainTelegram "ProxyMaster_v2/internal/domain/telegram"
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Client — это обертка над стандартной библиотекой tgbotapi.
// Он хранит в себе подключение к API и умеет отправлять сообщения.
type Client struct {
	api    *tgbotapi.BotAPI
	logger logger.Logger
}

// NewTelegramClient создает нового клиента для Telegram.
// token: токен бота, который мы получили от BotFather.
func NewTelegramClient(token string, logger logger.Logger) (*Client, error) {
	// Инициализируем библиотеку с токеном
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	return &Client{
		api:    api,
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
) {
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
			)
			if err != nil {
				c.logger.Error("Ошибка обработки callback", logger.Field{Key: "error", Value: err})
			}

			// Обязательно отвечаем Telegram, что мы приняли нажатие (иначе у юзера будет крутиться "часики")
			if _, err := c.api.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "")); err != nil {
				c.logger.Error("Ошибка ответа на callback query", logger.Field{Key: "error", Value: err})
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
			err := domainTelegram.ProcessCommand(c,
				update.Message.Chat.ID,
				update.Message.Text,
				firstName,
				remnawaveClient,
				userRepo)
			if err != nil {
				c.logger.Error("Ошибка обработки команды", logger.Field{Key: "error", Value: err})
			}
		}
	}
}

// ShowView отправляет сообщение с нужной клавиатурой в зависимости от типа
// Если messageID > 0, то сообщение редактируется. Иначе — отправляется новое.
func (c *Client) ShowView(chatID int64, messageID int, viewType string, data string) error {
	var text string
	var keyboard tgbotapi.InlineKeyboardMarkup

	switch viewType {
	case "download_app":
		text, keyboard = c.handleDownloadAppView()
	case "ios_region":
		text, keyboard = c.handleIosRegionView()
	case "tariffs":
		text, keyboard = c.handleTariffsView()
	case "topup":
		text, keyboard = c.handleTopupView()
	case "payment":
		text, keyboard = c.handlePaymentView(data)
	case "check_payment":
		text, keyboard = c.handleCheckPaymentView(data)
	case "profile":
		text, keyboard = c.handleProfileView(data)
	case "connect":
		text, keyboard = c.handleConnectView(data)
	case "subscription_result":
		text, keyboard = c.handleSubscriptionResultView(data)
	case "main":
		text, keyboard = c.handleMainView(data)
	default:
		return fmt.Errorf("неизвестный тип view: %s", viewType)
	}

	if messageID > 0 {
		// Редактируем существующее сообщение
		msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
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
	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки нового сообщения: %w", err)
	}

	return nil
}

func (c *Client) handleCheckPaymentView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	// data format: "url|transactionID"
	parts := strings.Split(data, "|")
	url := parts[0]
	transactionID := ""
	if len(parts) > 1 {
		transactionID = parts[1]
	}
	text := "Ссылка на оплату сформирована. После оплаты нажмите 'Проверить платеж'."
	keyboard := c.checkPaymentKeyboard(url, transactionID)

	return text, keyboard
}

func (c *Client) handleSubscriptionResultView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	return data, c.profileKeyboard()
}

func (c *Client) handleDownloadAppView() (string, tgbotapi.InlineKeyboardMarkup) {
	return "Какое у вас устройство?", c.downloadAppKeyboard()
}

func (c *Client) handleIosRegionView() (string, tgbotapi.InlineKeyboardMarkup) {
	text := "Выберите какой у вас регион App Store.\n\n" +
		"Если не знаете, попробуйте сначала RU, если выдаст ошибку, то 'другие регионы'"

	return text, c.iosRegionKeyboard()
}

func (c *Client) handleTariffsView() (string, tgbotapi.InlineKeyboardMarkup) {
	return "Выберите тариф подписки:", c.tariffsKeyboard()
}

func (c *Client) handleTopupView() (string, tgbotapi.InlineKeyboardMarkup) {
	return "💰 Выберите сумму для пополнения баланса:", c.topupKeyboard()
}

func (c *Client) handlePaymentView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	return "Выберите способ оплаты:", c.paymentKeyboard(data)
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

func (c *Client) handleMainView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	var text string
	if data != "" {
		text = data
	} else {
		text = "🌟 Добро пожаловать."
	}

	return text, c.mainKeyboard()
}

func (c *Client) handleProfileView(data string) (string, tgbotapi.InlineKeyboardMarkup) {
	parts := strings.SplitN(data, "|", 3)
	userID := ""
	balance := "0"
	extraDevices := "0"
	if len(parts) > 0 {
		userID = parts[0]
	}
	if len(parts) > 1 {
		balance = parts[1]
	}
	if len(parts) > 2 {
		extraDevices = parts[2]
	}

	extraDevicesInt, err := strconv.Atoi(extraDevices)
	if err != nil {
		extraDevicesInt = 0
	}

	text := fmt.Sprintf(
		"ID пользователя: %s\nБаланс: %s ₽\nДоп. устройств: %d\nДоп. списание: %d ₽/мес",
		userID,
		balance,
		extraDevicesInt,
		extraDevicesInt*50,
	)
	keyboard := c.profileKeyboard()

	return text, keyboard
}

// SendMessage отправляет текстовое сообщение пользователю.
// Также всегда прикрепляет главную клавиатуру, чтобы она не пропадала.
// Реализует метод интерфейса domain.MessageSender.
func (c *Client) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)

	// Всегда показываем главное меню под сообщением
	msg.ReplyMarkup = c.mainKeyboard()

	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения: %w", err)
	}

	return nil
}

// mainKeyboard создает структуру кнопок для главного меню.
// Используем Inline кнопки (прозрачные, под сообщением).
func (c *Client) mainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Скачать приложение", "download_app"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Подключить (Happ)", "btn_connect"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Пополнить баланс", "btn_balance"),
			tgbotapi.NewInlineKeyboardButtonData("📦 Тарифы", "btn_tariffs"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Увеличение лимитов", "btn_unlimits"),
			tgbotapi.NewInlineKeyboardButtonURL("🛟 Поддержка", "https://t.me/bloknotanet"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧾 Информация о сервисе", "btn_info"),
		),
	)
}
