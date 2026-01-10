// Package telegram отвечает за взаимодействие с Telegram API
package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	domainTelegram "ProxyMaster_v2/internal/domain/telegram"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Client — это обертка над стандартной библиотекой tgbotapi.
// Он хранит в себе подключение к API и умеет отправлять сообщения.
type Client struct {
	api *tgbotapi.BotAPI
}

// NewTelegramClient создает нового клиента для Telegram.
// token: токен бота, который мы получили от BotFather.
func NewTelegramClient(token string) (*Client, error) {
	// Инициализируем библиотеку с токеном
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	return &Client{
		api: api,
	}, nil
}

// Start — это "сердце" бота. Метод запускает бесконечный цикл,
// который слушает обновления от Telegram (сообщения, нажатия кнопок)
// и передает их в бизнес-логику (domain).
func (c *Client) Start(remnawaveClient domain.RemnawaveClient, subscriptionService domain.SubscriptionService, paymentGateway domain.PaymentGateway, userRepo *database.UserStorage) {
	// Настраиваем конфигурацию получения обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60 // Ждем 60 секунд новых сообщений (long polling)

	// Получаем канал, в который будут падать обновления
	updates, err := c.api.GetUpdatesChan(u)
	if err != nil {
		fmt.Printf("Ошибка получения обновлений: %v\n", err)
		return
	}

	// Читаем обновления из канала в бесконечном цикле
	for update := range updates {
		// 1. Обработка нажатий на инлайн-кнопки (CallbackQuery)
		if update.CallbackQuery != nil {
			// Передаем нажатие в слой бизнес-логики, включая ID сообщения для редактирования
			domainTelegram.ProcessCallback(
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

			// Обязательно отвечаем Telegram, что мы приняли нажатие (иначе у юзера будет крутиться "часики")
			c.api.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			continue
		}

		// 2. Обработка обычных текстовых сообщений
		if update.Message != nil {
			firstName := ""
			if update.Message.From != nil {
				firstName = update.Message.From.FirstName
			}

			// Передаем текст сообщения в слой бизнес-логики
			domainTelegram.ProcessCommand(c,
				update.Message.Chat.ID,
				update.Message.Text,
				firstName,
				remnawaveClient,
				userRepo)
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
		text = "Какое у вас устройство?"
		keyboard = c.downloadAppKeyboard()
	case "ios_region":
		text = "Выберите какой у вас регион App Store.\n\nЕсли не знаете, попробуйте сначала RU, если выдаст ошибку, то 'другие регионы'"
		keyboard = c.iosRegionKeyboard()
	case "tariffs":
		text = "Выберите тариф подписки:"
		keyboard = c.tariffsKeyboard()
	case "topup":
		text = "💰 Выберите сумму для пополнения баланса:"
		keyboard = c.topupKeyboard()
	case "payment":
		text = "Выберите способ оплаты:"
		keyboard = c.paymentKeyboard(data)
	case "check_payment":
		// data format: "url|transactionID"
		parts := strings.Split(data, "|")
		url := parts[0]
		transactionID := ""
		if len(parts) > 1 {
			transactionID = parts[1]
		}
		text = "Ссылка на оплату сформирована. После оплаты нажмите 'Проверить платеж'."
		keyboard = c.checkPaymentKeyboard(url, transactionID)
	case "profile":
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

		text = fmt.Sprintf(
			"ID пользователя: %s\nБаланс: %s ₽\nДоп. устройств: %d\nДоп. списание: %d ₽/мес",
			userID,
			balance,
			extraDevicesInt,
			extraDevicesInt*50,
		)
		keyboard = c.profileKeyboard()
	case "connect":
		if data == "" {
			text = "Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку."
		} else {
			text = "Ваша ссылка для подключения:\n" + data
		}
		keyboard = c.connectKeyboard()
	case "subscription_result":
		text = data
		keyboard = c.profileKeyboard()
	case "main":
		if data != "" {
			text = data
		} else {
			text = "🌟 Добро пожаловать."
		}
		keyboard = c.mainKeyboard()
	default:
		return fmt.Errorf("неизвестный тип view: %s", viewType)
	}

	if messageID > 0 {
		// Редактируем существующее сообщение
		msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		msg.ReplyMarkup = &keyboard
		_, err := c.api.Send(msg)
		return err
	} else {
		// Отправляем новое сообщение
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = keyboard
		_, err := c.api.Send(msg)
		return err
	}
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
			tgbotapi.NewInlineKeyboardButtonData("👤 Личный кабинет", "btn_profile"),
			tgbotapi.NewInlineKeyboardButtonURL("🛟 Поддержка", "https://t.me/bloknotanet"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧾 Информация о сервисе", "btn_info"),
		),
	)
}
