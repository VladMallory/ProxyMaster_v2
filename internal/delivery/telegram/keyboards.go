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

// checkPaymentKeyboard генерирует клавиатуру для проверки платежа.
func (c *Client) checkPaymentKeyboard(url, transactionID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗 Оплатить", url),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Проверить платеж", "btn_check_payment_"+transactionID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 В главное меню", "btn_back"),
		),
	)
}
func (c *Client) paymentKeyboard(amount string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 СБП", "btn_pay_sbp_"+amount),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🪙 Криптовалюта", "btn_pay_crypto_"+amount),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад к тарифам", "btn_balance"), // Возвращаем к выбору тарифов
		),
	)
}
