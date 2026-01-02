// Package telegrambot обрабатывает клавиатуру от пользователя. В данном случае
// сколько на месяцев он хочет подписку.
package telegrambot

import (
	"errors"
	"fmt"
	"log/slog"
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
func NewCallbackHandler(
	subService domain.SubscriptionService,
	telegramSupport string,
	remnawaveClient domain.RemnawaveClient,
) *CallbackHandler {
	slog.Info("Создан экземпляр подписачного сервиса")

	return &CallbackHandler{
		subService:      subService,
		telegramSupport: telegramSupport,
		remnawaveClient: remnawaveClient,
	}
}

// mainMenu метод для обработки главного меню
func (h *CallbackHandler) mainMenu(update tgbotapi.Update, bot *tgbotapi.BotAPI, userID int) error {
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
func (h *CallbackHandler) tariffs(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
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
func (h *CallbackHandler) profile(update tgbotapi.Update, bot *tgbotapi.BotAPI, userID int) error {
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
func (h *CallbackHandler) support(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
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
func (h *CallbackHandler) info(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
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
func (h *CallbackHandler) topupBalance(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
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
func (h *CallbackHandler) agreement(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
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
func (h *CallbackHandler) createUser(bot *tgbotapi.BotAPI, userID int, data string) error {
	monthsStr := strings.TrimPrefix(data, "create_user_")
	months, err := strconv.Atoi(monthsStr)
	if err != nil {
		return fmt.Errorf("неверный формат месяцев: %s", monthsStr)
	}

	// Вызываем сервис подписки
	resultMsg, err := h.subService.ActivateSubscription(int64(userID), months)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) {
			// Если недостаточно средстав, предлагаем пополнить
			msg := tgbotapi.NewMessage(
				int64(userID),
				fmt.Sprintf("❌Пожалуйста, пополните баланс в личном кабинете."),
			)

			// Добавляем кнопку пополнения
			keyboard := telegram.NewProfileKeyboard()
			msg.ReplyMarkup = keyboard

			_, err := bot.Send(msg)
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
