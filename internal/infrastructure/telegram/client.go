package telegram

import (
	"fmt"
	"log"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// добавить storage в структуру позже
type Bot struct {
	*tgbotapi.BotAPI
}

// Структура нового бота через тайп эмбендинг
// теперь через структуру *Bot мы получаем
// всё что даёт структура *tgbotapi.BotAPI
// благодаря этому можем вешать на нашу собственную структуру
// *Bot свои методы
func NewBot(token string) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panicf("NewTelegramBotError: %v", err)
	}

	return &Bot{
		bot,
	}, nil
}

// Run - запуск телеграм бота. Слушает что приходит
func (b *Bot) Run() error {
	// получаем канал обновлений
	updates, err := b.HandleUpdates()
	if err != nil {
		slog.Error("ошибка при запуске прослушивания", "error", err)
	}

	log.Println("Бот запущен")

	// читаем сообщения из канала в бесконечном цикле
	for update := range updates {
		// если пришло обновления, но нет сообщения, пропускаем
		if update.Message == nil {
			continue
		}

		// slog.Info("Сообщение: ", "test", update.Message.Text, "user", update.Message.From.ID)

		log.Println("tgBot:", update.Message.From.UserName, update.Message.Text)

		// сообщение идет в роутинг команд
		err := b.CommandHandler(update)
		if err != nil {
			return err
		}
	}
	return nil
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
func (bot *Bot) HandleUpdates() (tgbotapi.UpdatesChannel, error) {

	// это настройка запроса. 0 - дает все с самого
	// начала и то что еще не обработал
	u := tgbotapi.NewUpdate(0)

	// сколько ждем
	u.Timeout = 60

	//получаем updates
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

// HandleCommands - роутинг команды. Принимает сообщение которое
// пришло в bot.Run() и решает что дальше с ним делать
func (bot *Bot) CommandHandler(update tgbotapi.Update) error {
	switch update.Message.Command() {
	case "start":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет")
		_, err := bot.Send(msg)
		if err != nil {
			return fmt.Errorf("BotHandleCommandStartError: %v", err)
		}
		return nil

	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Не знаю такой команды")
		_, err := bot.Send(msg)
		if err != nil {
			return fmt.Errorf("BotHandleCommandUnknownError: %v", err)
		}
	}

	return nil
}
