package telegram

import (
	"log"
	"strings"

	"gopkg.in/telebot.v4"
)

// Setup инициализация бота.
func Setup(bot *telebot.Bot, registry *Registry, start *StartHandler) {
	bot.Handle("/start", start.HandleStart)
	bot.Handle(&telebot.Btn{Unique: "platform_back"}, start.HandleBack)
	bot.Handle(&telebot.Btn{Unique: "users_back"}, start.HandleBack)

	for _, c := range registry.contributorsSnapshot() {
		for unique, h := range c.Handlers() {
			btn := &telebot.Btn{Unique: unique}
			bot.Handle(btn, h)
		}
	}

	// Старые меню присылают callback без \f-префикса, поэтому telebot не находит
	// для них Unique-обработчик и отдаёт сюда. Ведём такие нажатия в актуальное /start.
	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		if !isLegacyPaymentCallback(c.Data()) {
			return c.Respond()
		}

		if err := c.Respond(); err != nil {
			return err
		}

		return start.HandleStart(c)
	})

	// Должен быть последним. Перехватит неизвестнный текст
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		return c.Send("Неизвестная команда, введите /start")
	})
}

// SetupCommands команды бота.
// isLegacyPaymentCallback узнаёт вход в оплату из меню старого бота.
// Старые callback-и приходили голой строкой, без \f-префикса telebot.
func isLegacyPaymentCallback(data string) bool {
	switch data {
	case "btn_balance", "topup_balance", "payment_menu":
		return true
	default:
		return strings.HasPrefix(data, "btn_topUp_")
	}
}

func SetupCommands(bot *telebot.Bot) {
	if err := bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Вызывать главное меню"},
	}); err != nil {
		log.Fatalln(err)
	}
}
