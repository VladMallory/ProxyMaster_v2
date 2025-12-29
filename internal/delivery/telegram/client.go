package telegram

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Command - интерфейс для всех команд бота, /start /help и прочих
// нужен, чтобы следовать принципам SOLID. Закрыт для изменений
// добавлять будем через мапу, так минимальные шансы что-то
// сломать из старого кода
type Command interface {
	// Name то какая строка. /start, /help и т.д.
	Name() string

	// Execute что делаем со строкой
	// 1. tgbotapi. Update - внутри Update лежит все что прислал пользователь
	// текст сообщения ("Привет", "/start"), кто он (ChatID, UserID), имя и т.д.
	// 2. tgbotapi. BotAPI - делает запросы в телеграм. Send, DeleteMessage, KickChatMember (выгнать) и т.д.
	Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error
}

// Client - зависимости для телеграм
type Client struct {
	// Само апи телеграмма
	bot *tgbotapi.BotAPI
	// Команды которые бот должен обработать. /start /help и т.д.
	commands map[string]Command
	// Обработчик кнопок
	callbackHandler func(tgbotapi.Update, *tgbotapi.BotAPI) error
}

// NewClient - экземпляр бота
func NewClient(bot *tgbotapi.BotAPI) *Client {
	fmt.Println("Создан экземпляр TelegramClient")
	return &Client{
		bot:      bot,
		commands: make(map[string]Command),
	}
}

// SetCallbackHandler устанавливает обработчик кнопок
func (c *Client) SetCallbackHandler(handler func(tgbotapi.Update, *tgbotapi.BotAPI) error) {
	c.callbackHandler = handler
}

// RegisterCommand - занимается регистрацией команд в боте
func (c *Client) RegisterCommand(cmd Command) {
	c.commands[cmd.Name()] = cmd
}

// Run - запуск цикла получения сообщения
func (c *Client) Run() {
	// получаем канал обновлений
	updates, err := c.initUpdatesChannel()
	if err != nil {
		slog.Error("ошибка при запуске прослушивания", "error", err)
		return
	}

	// читаем сообщения из канала в бесконечном цикле
	for update := range updates {
		// Если пришла команда, обрабатываем ее
		if update.Message != nil {
			fmt.Println("telegram message:", update.Message.From.ID, update.Message.Text)
			if update.Message.IsCommand() {
				c.handleUpdate(update)
			}
		}

		// Если пришел callback (кнопка), обрабатываем ее
		if update.CallbackQuery != nil {
			fmt.Println("telegram callback:", update.CallbackQuery.From.ID, update.CallbackQuery.Data)
			c.handleCallback(update)
			continue
		}
	}
}

func (c *Client) handleCallback(update tgbotapi.Update) {
	if c.callbackHandler != nil {
		if err := c.callbackHandler(update, c.bot); err != nil {
			slog.Error("ошибка в callback handler", "error", err)
		}
	}
}

// HandleCommands роутинг команды. Принимает сообщение которое
// пришло в Run() и решает что дальше с ним делать
func (c *Client) handleUpdate(update tgbotapi.Update) {
	cmdName := update.Message.Command()

	command, exists := c.commands[cmdName]

	if exists {
		if err := command.Execute(update, c.bot); err != nil {
			slog.Error("пользователь существует", "command", cmdName, "error", err)
		}
	}
}

// initUpdatesChannel - это как уши. Он слушает что приходит
// благодаря этому методу программа ждет сообщение и не завершается
// Есть два подхода
// ---
// 1. обычный подход Short Polling: бегаешь к почтовому ящику каждые 5 секунд,
// открываешь его и проверяешь. Пусто. Бежишь назад. Через 5 секунд снова
// Это лишние обращения к процессору и серверам телеги
// ---
// 2. подход Long Polling: подходишь к почтовому ящику, открываешь
// его. Стоишь и ждешь так 60 секунд если письмо есть,
// берем. Если не нет, то закрываем ящик, а
// потом опять открываем и ждем 60 секунд
func (c *Client) initUpdatesChannel() (tgbotapi.UpdatesChannel, error) {
	// Это настройка запроса. 0 - дает все с самого
	// начала и то что еще не обработал
	u := tgbotapi.NewUpdate(0)

	// сколько ждем
	u.Timeout = 60

	return c.bot.GetUpdatesChan(u)
}
