package keyboard

import (
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"gopkg.in/telebot.v4"
)

type Keyboard struct {
	supportURL string
}

func New(supportURL string) *Keyboard {
	return &Keyboard{supportURL: supportURL}
}

// Start клавиатура главного меню /start.
func (k *Keyboard) Start(users subdomain.User) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnDownload := menu.Data("📲 Скачать приложение", "btn_download")
	btnURL := menu.URL("🚀 Подключиться", users.URL)
	btnSupport := menu.URL("🛟 Поддержка", k.supportURL)

	menu.Inline(
		menu.Row(btnDownload),
		menu.Row(btnURL),
		menu.Row(btnSupport),
	)

	return menu
}

// DownloadApps клавиатура выбора платформы.
func (k *Keyboard) DownloadApps() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnIOS := menu.Data("🍎 iOS", "dl_ios")
	btnAndroid := menu.Data(
		"🤖 Android",
		"dl_android",
	)
	btnLinux := menu.Data("🐧 Linux", "dl_linux")
	btnWindows := menu.URL(
		"🪟 Windows",
		"https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe",
	)
	btnMacOS := menu.Data("💻 macOS", "dl_macos")
	btnRouter := menu.Data("📡 Роутер", "dl_router")
	btnBack := menu.Data("🏠 Главное меню", "btn_back")

	menu.Inline(
		menu.Row(btnIOS, btnAndroid),
		menu.Row(btnLinux, btnWindows, btnMacOS),
		menu.Row(btnRouter),
		menu.Row(btnBack),
	)

	return menu
}

// IOS подменю выбора App Store.
func (k *Keyboard) IOS() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnRu := menu.URL(
		"🇷🇺 App Store Россия",
		"https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6788279553",
	)
	btnOther := menu.URL(
		"🌍 App Store другие регионы",
		"https://apps.apple.com/us/app/happ-proxy-utility/id6504287215",
	)
	btnBack := menu.Data("← Назад", "btn_back_platforms")

	menu.Inline(
		menu.Row(btnRu),
		menu.Row(btnOther),
		menu.Row(btnBack),
	)

	return menu
}

func (k *Keyboard) Android() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnGooglePlay := menu.URL(
		"▶️ Google Play",
		"https://play.google.com/store/apps/details?id=com.happproxy",
	)
	btnDirectDownload := menu.URL(
		"📦 Скачать напрямую",
		"https://play.google.com/store/apps/details?id=com.happproxy",
	)
	btnBack := menu.Data("← Назад", "btn_back_platforms")

	menu.Inline(
		menu.Row(btnGooglePlay, btnDirectDownload),
		menu.Row(btnBack),
	)

	return menu
}

func (k *Keyboard) Linux() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnDebian := menu.URL(
		"🍃 Ubuntu/Debian/Mint",
		"https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.deb",
	)
	btnFedora := menu.URL(
		"🧊 Fedora",
		"https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.rpm",
	)
	btnArch := menu.URL(
		"🌀 Arch",
		"https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.linux.x64.pkg.tar.zst",
	)

	btnBack := menu.Data("← Назад", "btn_back_platforms")

	menu.Inline(
		menu.Row(btnDebian, btnFedora),
		menu.Row(btnArch),
		menu.Row(btnBack),
	)

	return menu
}

func (k *Keyboard) Macos() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	btnRu := menu.URL(
		"🇷🇺 App Store Россия",
		"https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6788279553",
	)
	btnOther := menu.URL(
		"🌍 App Store другие регионы",
		"https://apps.apple.com/us/app/happ-proxy-utility/id6504287215",
	)
	btnDirectDownload := menu.URL(
		"📦 Скачать DMG",
		"https://github.com/Happ-proxy/happ-desktop/releases/latest/download/Happ.macOS.universal.dmg",
	)

	btnBack := menu.Data("← Назад", "btn_back_platforms")

	menu.Inline(
		menu.Row(btnRu, btnOther),
		menu.Row(btnDirectDownload),
		menu.Row(btnBack),
	)

	return menu
}

func (k *Keyboard) Router() *telebot.ReplyMarkup {
	menu := k.Back()

	return menu
}

// Back одна кнопка возврата в главное меню.
func (k *Keyboard) Back() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnBack := menu.Data("🏠 Главное меню", "btn_back")

	menu.Inline(
		menu.Row(btnBack),
	)

	return menu
}
