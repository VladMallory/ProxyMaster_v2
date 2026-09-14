package telegram

import (
	"log"

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

	// Должен быть последним. Перехватит неизвестнный текст
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		return c.Send("Неизвестная команда, введите /start")
	})
}

// SetupCommands команды бота.
func SetupCommands(bot *telebot.Bot) {
	if err := bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Вызывать главное меню"},
	}); err != nil {
		log.Fatalln(err)
	}
}
