// Package telegrambot обрабатывает команды от пользователя.
// В данном случае /start
package telegrambot

import (
	"log"
	"log/slog"
	"strconv"

	"ProxyMaster_v2/internal/delivery/telegram"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartCommand это /start
type StartCommand struct {
	kbBuilder *telegram.KeyboardBuilder
	// Передаем ссылку на телеграмм
	telegramSupport string

	remnawaveClient domain.RemnawaveClient
}

// NewStartCommand is constructor for start struct
func NewStartCommand(
	kb *telegram.KeyboardBuilder,
	telegramSupport string,
	remnawaveClient domain.RemnawaveClient,
) *StartCommand {
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
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать в ProxyMaster! Выберите раздел:")

	urlSubscription := service.GetURLSubscription(s.remnawaveClient, strconv.Itoa(update.Message.From.ID))

	// Отправляем клавиатуру с поддержкой
	msg.ReplyMarkup = telegram.NewMainMenuKeyboard(s.telegramSupport, urlSubscription)

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("ошибка отправки сообщения: %v", err)
		slog.Error(
			"ошибка отправки сообщения",
			"err_msg", err,
		)
	}

	return nil
}
