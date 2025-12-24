package telegram

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// добавить storage в структуру позже
type Handler struct {
	bot *tgbotapi.BotAPI
}

func NewHandler(bot *tgbotapi.BotAPI) *Handler {
	return &Handler{
		bot: bot,
	}
}

// функция роутинга бота
func (h *Handler) HandleCommands(update tgbotapi.Update) {
	switch update.Message.Command() {
	case "start":
		//вызов метода обработки команды старт
		return
	}
}

func (h *Handler) ListenForMessages() tgbotapi.UpdatesChannel {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := h.bot.GetUpdatesChan(u)
	if err != nil {
		slog.Error(
			"Failed to get updates chan",
		)
	}
	return updates

}
