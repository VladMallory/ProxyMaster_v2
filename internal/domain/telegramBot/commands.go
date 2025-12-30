// package telegramBot обрабатывает команды от пользователя.
// В данном случае /start
package telegramBot

import (
	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/service"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartCommand это /start
type StartCommand struct {
	kbBuilder *telegram.KeyboardBuilder
	// Передаем ссылку на телеграмм
	telegramSupport string

	remnawaveClient domain.RemnawaveClient
}

func NewStartCommand(kb *telegram.KeyboardBuilder, telegramSupport string, remnawaveClient domain.RemnawaveClient) *StartCommand {
	return &StartCommand{
		kbBuilder:       kb,
		telegramSupport: telegramSupport,
		remnawaveClient: remnawaveClient,
	}
}

// Name возвращаем /start
func (s *StartCommand) Name() string {
	return "start"
}

// Execute то как идет обработка команд
func (s *StartCommand) Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать в ShadowFade! Выберите раздел:")

	urlSubscription := service.GetUrlSubscription(s.remnawaveClient, strconv.Itoa(update.Message.From.ID))

	// Отправляем клавиатуру с поддержкой
	msg.ReplyMarkup = telegram.NewMainMenuKeyboard(s.telegramSupport, urlSubscription)

	_, err := bot.Send(msg)

	return err
}
