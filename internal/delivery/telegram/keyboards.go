// Package telegram отвечает за создание клавиатур для телеграм бота
package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

// NewTrafficKeyboard создает клавиатуру с выбором трафика
func NewTrafficKeyboard() tgbotapi.InlineKeyboardMarkup {
	// создаем кнопки
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 месяц", "create_user_1"),
			tgbotapi.NewInlineKeyboardButtonData("3 месяца", "create_user_3"),
			tgbotapi.NewInlineKeyboardButtonData("6 месяцев", "create_user_6"),
		),
	)
	return keyboard
}
