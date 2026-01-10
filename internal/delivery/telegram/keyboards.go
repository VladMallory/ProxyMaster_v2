package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	androidAppURL        = "https://play.google.com/store/apps/details?id=com.happproxy"
	iosMacRuAppURL       = "https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973"
	iosMacOtherRegionURL = "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215"
	windowsAppURL        = "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe"
	linuxAppURL          = "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.deb"
)

func (c *Client) downloadAppKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🍎 iOS", "btn_ios_menu"),
			tgbotapi.NewInlineKeyboardButtonURL("🍏 Android", androidAppURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🪟 Windows", windowsAppURL),
			tgbotapi.NewInlineKeyboardButtonData("💻 macOS", "btn_macos_menu"),
			tgbotapi.NewInlineKeyboardButtonURL("🐧 Linux", linuxAppURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главная", "btn_back"),
		),
	)
}

func (c *Client) iosRegionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🍎Регион: RU", iosMacRuAppURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🍎 Другие регионы", iosMacOtherRegionURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

// tariffsKeyboard генерирует Inline-клавиатуру с вариантами подписки.
// Кнопки содержат:
// 1. Текст, который видит пользователь (например, "📅 1 Месяц - 100₽")
// 2. Data - скрытые данные, которые бот получит при нажатии (например, "btn_tariff_1")
func (c *Client) tariffsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 1 месяц - 100₽", "btn_sub_tariff_1"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 2 месяца - 200₽", "btn_sub_tariff_2"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 3 месяца - 300₽", "btn_sub_tariff_3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

func (c *Client) topupKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 100₽", "btn_topup_100"),
			tgbotapi.NewInlineKeyboardButtonData("💰 200₽", "btn_topup_200"),
			tgbotapi.NewInlineKeyboardButtonData("💰 300₽", "btn_topup_300"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 500₽", "btn_topup_500"),
			tgbotapi.NewInlineKeyboardButtonData("💰 700₽", "btn_topup_700"),
			tgbotapi.NewInlineKeyboardButtonData("💰 1000₽", "btn_topup_1000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

func (c *Client) profileKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить устройство (+50₽/мес)", "btn_add_device"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("♻️ Сбросить доп. устройства", "btn_reset_devices"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 В главное меню", "btn_back"),
		),
	)
}

func (c *Client) connectKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
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

// deviceLimitsKeyboard генерирует Inline-клавиатуру для меню лимитов устройств.
// Позволяет пользователю выбрать между просмотром профиля или управлением лимитами.
func (c *Client) deviceLimitsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Лимиты устройств", "btn_profile"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Лимиты трафика", "btn_traffic_limits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 В главное меню", "btn_back"),
		),
	)
}

// trafficLimitsKeyboard генерирует Inline-клавиатуру для меню лимитов трафика.
// Позволяет пользователю выбрать между просмотром профиля или управлением лимитами.
func (c *Client) trafficLimitsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_unlimits"),
		),
	)
}
