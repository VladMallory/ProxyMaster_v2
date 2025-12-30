// package telegramBot обрабатывает команды от пользователя.
// В данном случае /start
package telegramBot

import (
	"ProxyMaster_v2/internal/delivery/telegram"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartCommand это /start
type StartCommand struct {
	kbBuilder *telegram.KeyboardBuilder
}

func NewStartCommand(kb *telegram.KeyboardBuilder) *StartCommand {
	return &StartCommand{
		kbBuilder: kb,
	}
}

// Name возвращаем /start
func (s *StartCommand) Name() string {
	return "start"
}

func (s *StartCommand) Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать, выберите тариф:")

	// options := []string{"1 месяц", "2 месяца", "3 месяца"}
	// msg.ReplyMarkup = s.kbBuilder.BuildFromSlice(options)

	msg.ReplyMarkup = telegram.NewTrafficKeyboard()

	_, err := bot.Send(msg)

	return err
}
