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
type command interface {
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
	commands map[string]command
}

// NewClient - экземпляр бота
func NewClient(bot *tgbotapi.BotAPI) *Client {
	fmt.Println("Создан экземпляр TelegramClient")
	return &Client{
		bot:      bot,
		commands: make(map[string]command),
	}
}

// RegisterCommand - занимается регистрацией команд в боте
func (c *Client) RegisterCommand(cmd command) {
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
		// если пришло обновления, но нет сообщения, пропускаем
		if update.Message == nil {
			continue
		}

		fmt.Println("telegram:", update.Message.From.ID, update.Message.Text)

		// сообщение идет в роутинг команд
		c.handleUpdate(update)
	}
}

// HandleCommands роутинг команды. Принимает сообщение которое
// пришло в Run() и решает что дальше с ним делать
func (c *Client) handleUpdate(update tgbotapi.Update) {
	cmdName := update.Message.Command()

	command, exists := c.commands[cmdName]

	if exists {
		if err := command.Execute(update, c.bot); err != nil {
			slog.Error("ошибка при выполнении команды", "command", cmdName, "error", err)
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

// --------- команды ---------

type StartCommand struct{}

func (s *StartCommand) Name() string {
	return "start"
}

func (s *StartCommand) Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет")
	_, err := bot.Send(msg)
	if err != nil {
		return err
	}
	return nil
}
