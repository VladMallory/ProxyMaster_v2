// nolint: perfsprint, golines
package telegram

import (
	"fmt"
	"log"
	"strconv"

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

func (c *Client) pricePerMonth(n int) string {
	pricePerMonth := c.cfg.PricePerMonth

	// Преобразуем в int
	price, err := strconv.Atoi(pricePerMonth)
	if err != nil {
		log.Fatal("ошибка преобразования int", err)
	}

	// Умножаем цену одного месяца на то которое пришло в n
	total := price * n

	return strconv.Itoa(total)
}

func (c *Client) topUpKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("💰 %s₽ - 1 месяц", c.pricePerMonth(1)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(1)),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("💰 %s₽ - 2 месяца", c.pricePerMonth(2)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(2)),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("💰 %s₽ - 3 месяца", c.pricePerMonth(3)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(3)),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("💰 %s₽ - 5 месяцев", c.pricePerMonth(5)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(5)),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

func (c *Client) profileKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"➕ Добавить устройство (+50₽/мес)",
				"btn_add_device",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"💳 Оплатить доп. устройства заранее",
				"btn_prepay_devices",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"🔌 Сбросить устройства от подписки",
				"btn_reset_devices",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 В главное меню", "btn_back"),
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
func (c *Client) checkPaymentKeyboard(url string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗 Оплатить", url),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 В главное меню", "btn_back"),
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
			tgbotapi.NewInlineKeyboardButtonData("🏠 В главное меню", "btn_back"),
		),
	)
}

// trafficLimitsKeyboard генерирует Inline-клавиатуру для меню лимитов трафика.
// Позволяет пользователю выбрать между просмотром профиля или управлением лимитами.
func (c *Client) trafficLimitsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, сбросить трафик за 50₽", "btn_reset_traffic_payment"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Нет, вернуться назад", "btn_unlimits"),
		),
	)
}

// serviceInfoKeyboard генерирует клавиатуру для раздела "Информация о сервисе".
func (c *Client) serviceInfoKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"Политика конфиденциальности",
				"btn_privacy_policy",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"Пользовательское соглашение",
				"btn_user_agreement",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "btn_back"),
		),
	)
}

// handleBackView показывает клавиатуру для возврата в главное меню.
func handleBackView() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 В главное меню", "btn_back"),
		),
	)
}
