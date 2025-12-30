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
	// Передаем ссылку на телеграмм
	telegramSupport string
}

func NewStartCommand(kb *telegram.KeyboardBuilder, telegramSupport string) *StartCommand {
	return &StartCommand{
		kbBuilder:       kb,
		telegramSupport: telegramSupport,
	}
}

// Name возвращаем /start
func (s *StartCommand) Name() string {
	return "start"
}

func (s *StartCommand) Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать в ShadowFade! Выберите раздел:")

	// Отправляем клавиатуру с поддержкой
	msg.ReplyMarkup = telegram.NewMainMenuKeyboard(s.telegramSupport)

	_, err := bot.Send(msg)

	return err
}
