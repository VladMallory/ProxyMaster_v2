// package telegramBot обрабатывает клавиатуру от пользователя. В данном случае
// сколько на месяцев он хочет подписку.
package telegramBot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// CallbackHandler то какие сервисы используем
type CallbackHandler struct {
	// subService сервис подписки
	subService      domain.SubscriptionService
	telegramSupport string
	remnawaveClient domain.RemnawaveClient
}

// NewCallbackHandler конструктор
func NewCallbackHandler(subService domain.SubscriptionService, telegramSupport string, remnawaveClient domain.RemnawaveClient) *CallbackHandler {
	fmt.Println("Создан экземпляр подписочного сервиса")

	return &CallbackHandler{
		subService:      subService,
		telegramSupport: telegramSupport,
		remnawaveClient: remnawaveClient,
	}
}

// Handle обработка входящего callback
func (h *CallbackHandler) Handle(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID

	// Отвечаем телеграму, что мы получили callback (чтобы часики пропали)
	callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := bot.AnswerCallbackQuery(callbackCfg); err != nil {
		return fmt.Errorf("ошибка ответа на callback: %w", err)
	}

	// Обработка различных действий
	switch {
	// === ГЛАВНОЕ МЕНЮ И НАВИГАЦИЯ ===
	case data == "main_menu":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "Добро пожаловать в ProxyMaster! Выберите раздел:")
		// Создаем клавиатуру с ссылкой на поддержку

		urlSubscription := service.GetUrlSubscription(h.remnawaveClient, strconv.Itoa(userID))
		keyboard := telegram.NewMainMenuKeyboard(h.telegramSupport, urlSubscription)

		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	case data == "tariffs":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "Выберите срок подписки:")
		keyboard := telegram.NewTariffsKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	case data == "profile":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, fmt.Sprintf("👤 Личный кабинет\nID: %d\nБаланс: 0.00 ₽", userID))
		keyboard := telegram.NewProfileKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	case data == "support":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, fmt.Sprintf("🆘 Поддержка\n\nЕсли у вас возникли вопросы, напишите нам: %s", h.telegramSupport))
		keyboard := telegram.NewBackToMenuKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	case data == "info":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "ℹ️ Информация о сервисе\n\nProxyMaster - лучший VPN сервис.")
		keyboard := telegram.NewInfoKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	case data == "topup_balance":
		// Заглушка для пополнения
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "💳 Выберите способ оплаты (в разработке):")
		keyboard := telegram.NewBackToMenuKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	// === КОНЕЧНЫЕ ДЕЙСТВИЯ ===
	// 1. Обработка запроса на пользовательское соглашение
	case data == "agreement":
		msg := tgbotapi.NewEditMessageText(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, "📜 Пользовательское соглашение\n\n1. Пункт первый\n2. Пункт второй")
		keyboard := telegram.NewBackToMenuKeyboard()
		msg.ReplyMarkup = &keyboard
		_, err := bot.Send(msg)
		return err

	// 2. Логика обработки создания подписки (create_user_{months})
	case strings.HasPrefix(data, "create_user_"):
		monthsStr := strings.TrimPrefix(data, "create_user_")
		months, err := strconv.Atoi(monthsStr)
		if err != nil {
			return fmt.Errorf("неверный формат месяцев: %s", monthsStr)
		}

		// Вызываем сервис подписки
		resultMsg, err := h.subService.ActivateSubscription(int64(userID), months)
		if err != nil {
			log.Printf("Ошибка активации подписки: %v", err)
			msg := tgbotapi.NewMessage(int64(userID), fmt.Sprintf("Произошла ошибка при обработке заказа, обратитесь в поддержку: %s .Ошибка: %s\n", h.telegramSupport, err))
			_, _ = bot.Send(msg)
			return err
		}

		// Отправляем успешный ответ пользователю
		msg := tgbotapi.NewMessage(int64(userID), resultMsg)
		_, err = bot.Send(msg)
		if err != nil {
			log.Println("ошибка отправки сообщения:", err)
		}

		return nil
	}

	return nil
}
