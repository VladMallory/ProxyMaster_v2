package telegram

import (
	"log"

	"gopkg.in/telebot.v4"
)

// RegisterFallback регистрирует общий обработчик неизвестных команд.
// Должен регистрироваться ПОСЛЕДНИМ, после всех контекстных RegisterRoutes,
// иначе перехватит любой текст раньше хендлеров с командами.
func RegisterFallback(bot *telebot.Bot) {
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		err := c.Send("Неизвестная команда, введите /start")
		if err != nil {
			log.Fatalln(err)
		}

		return nil
	})
}
