package telegram

import (
	"fmt"
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

// Run - запуск телеграм бота. Слушает что приходит
func (h *Handler) Run() {
	// получаем канал обновлений
	updates, err := h.ListenForMessages()
	if err != nil {
		slog.Error("ошибка при запуске прослушивания", "error", err)
	}

	// читаем сообщения из канала в бесконечном цикле
	for update := range updates {
		// если пришло обновления, но нет сообщения, пропускаем
		if update.Message == nil {
			continue
		}

		// slog.Info("Сообщение: ", "test", update.Message.Text, "user", update.Message.From.ID)

		fmt.Println("telegram:", update.Message.From.ID, update.Message.Text)

		// сообщение идет в роутинг команд
		h.HandleCommands(update)
	}
}

// HandleCommands - роутинг команды. Принимает сообщение которое
// пришло в Run() и решает что дальше с ним делать
func (h *Handler) HandleCommands(update tgbotapi.Update) {
	switch update.Message.Command() {
	case "start":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет")
		h.bot.Send(msg)
		return
	}
}

// ListenForMessages - это как уши. Он слушает что приходит
// благодаря этому методу программа ждет сообщение и не завершается
// Есть два подхода
// ---
// 1. обычный подход Short Polling: бегаешь к почтовому ящику каждые 5 секунд,
// открываешь его и проверяешь. Пусто. Бежишь назад. Через 5 секунд снова
// Это лишние обращения к процессору и серверам телеги
// ---
// 2. подход Long Polling: подходишь к почтовому ящику, открываешь
// его. стоишь и ждешь так 60 секунд если письмо есть,
// берем. Если не нет, то закрываем ящик, а
// потом опять открываем и ждем 60 секунд
func (h *Handler) ListenForMessages() (tgbotapi.UpdatesChannel, error) {
	// это настройка запроса. 0 - дает все с самого
	// начала и то что еще не обработал
	u := tgbotapi.NewUpdate(0)

	// сколько ждем
	u.Timeout = 60

	//
	updates, err := h.bot.GetUpdatesChan(u)
	if err != nil {
		return nil, err
	}

	return updates, nil
}
