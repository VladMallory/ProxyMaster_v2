// nolint: perfsprint, golines
package telegram

import (
	"fmt"
	"strconv"

	tgintegration "github.com/VladMallory/ProxyMaster_v2/internal/integrations/telegram"
)

const (
	androidAppURL        = "https://play.google.com/store/apps/details?id=com.happproxy"
	iosMacRuAppURL       = "https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973"
	iosMacOtherRegionURL = "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215"
	windowsAppURL        = "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe"
	linuxAppURL          = "https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.deb"
)

func (c *Client) downloadAppKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton("🍎 iOS", "btn_ios_menu"),
			tgintegration.NewURLButton("🍏 Android", androidAppURL),
		},
		{
			tgintegration.NewURLButton("🪟 Windows", windowsAppURL),
			tgintegration.NewButton("💻 macOS", "btn_macos_menu"),
			tgintegration.NewURLButton("🐧 Linux", linuxAppURL),
		},
		{
			tgintegration.NewButton("🏠 Главная", "btn_back"),
		},
	}
}

func (c *Client) iosRegionKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewURLButton("🍎Регион: RU", iosMacRuAppURL),
		},
		{
			tgintegration.NewURLButton("🍎 Другие регионы", iosMacOtherRegionURL),
		},
		{
			tgintegration.NewButton("🔙 Назад", "btn_back"),
		},
	}
}

func (c *Client) pricePerMonth(n int) string {
	pricePerMonth := c.cfg.PricePerMonth

	price, err := strconv.Atoi(pricePerMonth)
	if err != nil {
		return ""
	}

	total := price * n

	return strconv.Itoa(total)
}

func (c *Client) topUpKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton(
				fmt.Sprintf("💰 %s₽ - 1 месяц", c.pricePerMonth(1)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(1)),
			),
			tgintegration.NewButton(
				fmt.Sprintf("💰 %s₽ - 2 месяца", c.pricePerMonth(2)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(2)),
			),
		},
		{
			tgintegration.NewButton(
				fmt.Sprintf("💰 %s₽ - 3 месяца", c.pricePerMonth(3)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(3)),
			),
			tgintegration.NewButton(
				fmt.Sprintf("💰 %s₽ - 5 месяцев", c.pricePerMonth(5)),
				fmt.Sprintf("btn_topUp_%s", c.pricePerMonth(5)),
			),
		},
		{
			tgintegration.NewButton("🔙 Назад", "btn_back"),
		},
	}
}

func (c *Client) profileKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton(
				fmt.Sprintf("➕ Добавить устройство (+%d₽/мес)", c.cfg.ExtraDevicePrice),
				"btn_add_device",
			),
		},
		{
			tgintegration.NewButton(
				"💳 Оплатить доп. устройства заранее",
				"btn_prepay_devices",
			),
		},
		{
			tgintegration.NewButton(
				"🔌 Отвязать устройства от подписки",
				"btn_reset_devices",
			),
		},
		{
			tgintegration.NewButton("🏠 В главное меню", "btn_back"),
		},
	}
}

func (c *Client) connectKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton("🔙 Назад", "btn_back"),
		},
	}
}

func (c *Client) checkPaymentKeyboard(url string) tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewURLButton("🔗 Оплатить", url),
		},
		{
			tgintegration.NewButton("🏠 В главное меню", "btn_back"),
		},
	}
}

func (c *Client) deviceLimitsKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton("📱 Лимиты устройств", "btn_profile"),
		},
		{
			tgintegration.NewButton("📊 Лимиты трафика", "btn_traffic_limits"),
		},
		{
			tgintegration.NewButton("🏠 В главное меню", "btn_back"),
		},
	}
}

func (c *Client) trafficLimitsKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton(fmt.Sprintf("✅ Да, сбросить трафик за %d₽", c.cfg.ResetTrafficPrice), "btn_reset_traffic_payment"),
		},
		{
			tgintegration.NewButton("❌ Нет, вернуться назад", "btn_unlimits"),
		},
	}
}

func (c *Client) serviceInfoKeyboard() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton(
				"Политика конфиденциальности",
				"btn_privacy_policy",
			),
		},
		{
			tgintegration.NewButton(
				"Пользовательское соглашение",
				"btn_user_agreement",
			),
		},
		{
			tgintegration.NewButton("🔙 Назад", "btn_back"),
		},
	}
}

func handleBackView() tgintegration.InlineKeyboard {
	return tgintegration.InlineKeyboard{
		{
			tgintegration.NewButton("🏠 В главное меню", "btn_back"),
		},
	}
}
