// Package telegram отвечает за создание клавиатур для телеграм бота
package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// KeyboardBuilder новый экземпляр.
type KeyboardBuilder struct{}

// NewKeyboardBuilder конструктор.
func NewKeyboardBuilder() *KeyboardBuilder {
	return &KeyboardBuilder{}
}

// BuildFromSlice создает клавиатуру из слайса строк
func (k *KeyboardBuilder) BuildFromSlice(options []string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, text := range options {
		btn := tgbotapi.NewInlineKeyboardButtonData(text, text)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// NewMainMenuKeyboard создает главное меню
func NewMainMenuKeyboard(telegramSupport, subscriptionURL string) tgbotapi.InlineKeyboardMarkup {
	// Если подписки нет (URL пустой), показываем предложение купить
	if subscriptionURL == "" {
		return tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📦 Оформить подписку", "tariffs"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("🆘 Поддержка", telegramSupport),
			),
		)
	}

	// Если есть подписка, показываем полное меню
	connectBtn := tgbotapi.NewInlineKeyboardButtonURL("🔗 Подключить", subscriptionURL)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			connectBtn,
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📦 Продлить подписку", "tariffs"),
			tgbotapi.NewInlineKeyboardButtonData("👤 Личный кабинет", "profile"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🆘 Поддержка", telegramSupport),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Инфо", "info"),
		),
	)
}

// NewTariffsKeyboard создает клавиатуру с выбором тарифов
func NewTariffsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 месяц", "create_user_1"),
			tgbotapi.NewInlineKeyboardButtonData("2 месяца", "create_user_2"),
			tgbotapi.NewInlineKeyboardButtonData("3 месяца", "create_user_3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "main_menu"),
		),
	)
}

// NewProfileKeyboard создает клавиатуру личного кабинета
func NewProfileKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Пополнить баланс", "topup_balance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "main_menu"),
		),
	)
}

// NewInfoKeyboard создает клавиатуру раздела информации
func NewInfoKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 Пользовательское соглашение", "agreement"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "main_menu"),
		),
	)
}

// NewBackToMenuKeyboard создает клавиатуру с кнопкой возврата в меню
func NewBackToMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "main_menu"),
		),
	)
	return keyboard
}
