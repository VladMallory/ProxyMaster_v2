// Package telegram отвечает за создание клавиатур для телеграм бота
package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type KeyboardBuilder struct{}

// KeyboardBuilder конструктор
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

// NewTrafficKeyboard создает клавиатуру с выбором трафика
func NewTrafficKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1 месяц", "create_user_1"),
			tgbotapi.NewInlineKeyboardButtonData("2 месяца", "create_user_2"),
			tgbotapi.NewInlineKeyboardButtonData("3 месяца", "create_user_3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 Пользовательское соглашение", "agreement"),
		),
	)
	return keyboard
}
