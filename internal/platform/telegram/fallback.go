package telegram

import (
	"gopkg.in/telebot.v4"
)

// RegisterFallback регистрирует общий обработчик неизвестных команд.
// Должен регистрироваться ПОСЛЕДНИМ, после всех контекстных RegisterRoutes,
// иначе перехватит любой текст раньше хендлеров с командами.
func RegisterFallback(bot *telebot.Bot) error {
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		return c.Send("Неизвестная команда, введите /start")
	})

	return nil
}
