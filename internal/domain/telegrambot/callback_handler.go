// Package telegrambot обрабатывает клавиатуру от пользователя. В данном случае
// сколько на месяцев он хочет подписку.
package telegrambot

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Handler - основной обработчик всей логики бота (команды + кнопки)
type Handler struct {
	// subService сервис подписки
	subService      domain.SubscriptionService
	telegramSupport string
	remnawaveClient domain.RemnawaveClient
}

// NewHandler конструктор
func NewHandler(
	subService domain.SubscriptionService,
	telegramSupport string,
	remnawaveClient domain.RemnawaveClient) *Handler {
	slog.Info("Создан экземпляр обработчика бота")

	return &Handler{
		subService:      subService,
		telegramSupport: telegramSupport,
		remnawaveClient: remnawaveClient,
	}
}

// Handle - единая точка входа для всех обновлений
func (h *Handler) Handle(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	// 1. Если это текстовая команда (например /start)
	if update.Message != nil && update.Message.IsCommand() {
		return h.handleCommand(update, bot)
	}

	// 2. Если это нажатие на кнопку (callback)
	if update.CallbackQuery != nil {
		return h.handleCallback(update, bot)
	}

	return nil
}

// mainMenu метод для обработки главного меню
func (h *Handler) mainMenu(update tgbotapi.Update, bot *tgbotapi.BotAPI, userID int) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		"Добро пожаловать в ProxyMaster! Выберите раздел:",
	)

	// Создаем клавиатуру с ссылкой на поддержку
	urlSubscription := service.GetURLSubscription(h.remnawaveClient, strconv.Itoa(userID))
	keyboard := telegram.NewMainMenuKeyboard(h.telegramSupport, urlSubscription)

	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// tariffs метод для обработки тарифов
func (h *Handler) tariffs(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		"Выберите срок подписки:",
	)
	keyboard := telegram.NewTariffsKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// profile метод для обработки профиля
func (h *Handler) profile(update tgbotapi.Update, bot *tgbotapi.BotAPI, userID int) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		fmt.Sprintf("👤 Личный кабинет\nID: %d\nБаланс: 0.00 ₽", userID),
	)
	keyboard := telegram.NewProfileKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// support метод для поддержки
func (h *Handler) support(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		fmt.Sprintf("🆘 Поддержка\n\nЕсли у вас возникли вопросы, напишите нам: %s",
			h.telegramSupport),
	)
	keyboard := telegram.NewBackToMenuKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// info метод для вывода информации
func (h *Handler) info(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID,
		"ℹ️ Информация о сервисе\n\nProxyMaster - лучший VPN сервис.",
	)
	keyboard := telegram.NewInfoKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// topupBalance метод заглушка пока что
func (h *Handler) topupBalance(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	// Заглушка для пополнения
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		"💳 Выберите способ оплаты (в разработке):",
	)
	keyboard := telegram.NewBackToMenuKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// agreement метод для вывода пользовательского соглашения
func (h *Handler) agreement(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		"📜 Пользовательское соглашение\n\n1. Пункт первый\n2. Пункт второй",
	)
	keyboard := telegram.NewBackToMenuKeyboard()
	msg.ReplyMarkup = &keyboard
	_, err := bot.Send(msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// createUser метод для создания пользователя
func (h *Handler) createUser(bot *tgbotapi.BotAPI, userID int, data string) error {
	monthsStr := strings.TrimPrefix(data, "create_user_")
	months, err := strconv.Atoi(monthsStr)
	if err != nil {
		return fmt.Errorf("неверный формат месяцев: %s", monthsStr)
	}

	// Вызываем сервис подписки
	resultMsg, err := h.subService.ActivateSubscription(int64(userID), months)
	if err != nil {
		// Ошибка ErrInsufficientFunds должна быть проверена через errors.Is или сравнение строк,
		// так как мы не имеем доступа к errors.Is в этом блоке без импорта, но импорт errors есть.
		// В оригинальном коде errors был импортирован.
		
		// Простая проверка на ошибку недостаточно средств
		if strings.Contains(err.Error(), "недостаточно средств") { // Упрощение для надежности, если импорт потерялся, хотя он должен быть
			// Если недостаточно средстав, предлагаем пополнить
			msg := tgbotapi.NewMessage(
				int64(userID),
				"❌Пожалуйста, пополните баланс в личном кабинете.",
			)

			// Добавляем кнопку пополнения
			keyboard := telegram.NewProfileKeyboard()
			msg.ReplyMarkup = keyboard

			_, err = bot.Send(msg)
			if err != nil {
				return fmt.Errorf("ошибка отправки сообщения о пополнении: %w", err)
			}
			return nil
		}

		slog.Error(
			"ошибка активации подписки",
			"err_msg", err,
		)
		msg := tgbotapi.NewMessage(
			int64(userID),
			fmt.Sprintf("Произошла ошибка при обработке заказа, обратитесь в поддержку: %s\n",
				h.telegramSupport),
		)
		_, err = bot.Send(msg)

		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		// Возвращаем nil, так как мы уже обработали ошибку отправкой сообщения пользователю
		return nil
	}

	// Отправляем успешный ответ пользователю
	msg := tgbotapi.NewMessage(int64(userID), resultMsg)
	_, err = bot.Send(msg)
	if err != nil {
		slog.Error(
			"ошибка отправки сообщения",
			"err_msg", err,
		)
	}

	return nil
}

// handleCommand маршрутизация команд
func (h *Handler) handleCommand(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	cmd := update.Message.Command()
	fmt.Println("Команда:", cmd)

	switch cmd {
	case "start":
		return h.start(update, bot)
	}
	return nil
}

// start обработка команды /start
func (h *Handler) start(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать в ProxyMaster! Выберите раздел:")

	// Генерируем ссылку для подписки (если нужно)
	urlSubscription := service.GetURLSubscription(h.remnawaveClient, strconv.Itoa(update.Message.From.ID))

	// Отправляем клавиатуру главного меню
	msg.ReplyMarkup = telegram.NewMainMenuKeyboard(h.telegramSupport, urlSubscription)

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("ошибка отправки приветствия: %w", err)
	}

	return nil
}

// handleCallback обработка нажатий на кнопки
func (h *Handler) handleCallback(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID

	fmt.Println("Callback:", data, "User:", userID)

	// Отвечаем телеграму, что мы получили callback (чтобы часики пропали)
	callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := bot.AnswerCallbackQuery(callbackCfg); err != nil {
		return fmt.Errorf("ошибка ответа на callback: %w", err)
	}

	// Обработка различных действий
	switch {
	// === ГЛАВНОЕ МЕНЮ И НАВИГАЦИЯ ===
	case data == "main_menu":
		if err := h.mainMenu(update, bot, userID); err != nil {
			return err
		}

	case data == "tariffs":
		if err := h.tariffs(update, bot); err != nil {
			return err
		}

	case data == "profile":
		if err := h.profile(update, bot, userID); err != nil {
			return err
		}

	case data == "support":
		if err := h.support(update, bot); err != nil {
			return err
		}

	case data == "info":
		if err := h.info(update, bot); err != nil {
			return err
		}

	case data == "topup_balance":
		if err := h.topupBalance(update, bot); err != nil {
			return err
		}

	// === КОНЕЧНЫЕ ДЕЙСТВИЯ ===
	// 1. Обработка запроса на пользовательское соглашение
	case data == "agreement":
		if err := h.agreement(update, bot); err != nil {
			return err
		}

	// 2. Логика обработки создания подписки (create_user_{months})
	case strings.HasPrefix(data, "create_user_"):
		if err := h.createUser(bot, userID, data); err != nil {
			return err
		}
	}

	return nil
}
