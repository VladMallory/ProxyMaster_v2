package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

// tariffsKeyboard генерирует Inline-клавиатуру с вариантами подписки.
// Кнопки содержат:
// 1. Текст, который видит пользователь (например, "📅 1 Месяц - 100₽")
// 2. Data - скрытые данные, которые бот получит при нажатии (например, "btn_tariff_1")
func (c *Client) tariffsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 1 Месяц - 100₽", "btn_tariff_1"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 3 Месяца - 270₽", "btn_tariff_3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

// paymentKeyboard генерирует Inline-клавиатуру с методами оплаты.
func (c *Client) paymentKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Банковская карта (RF)", "btn_pay_card"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🪙 Криптовалюта", "btn_pay_crypto"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад к тарифам", "btn_balance"), // Возвращаем к выбору тарифов
		),
	)
}
