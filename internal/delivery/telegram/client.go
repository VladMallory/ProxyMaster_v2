package telegram

import (
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/infrastructure/remnawave"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"

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

	// Задаем зависимость от клиента Remnawave
	remna domain.RemnawaveClient
}

// NewClient - экземпляр бота
func NewClient(bot *tgbotapi.BotAPI, remna domain.RemnawaveClient) *Client {
	fmt.Println("Создан экземпляр TelegramClient")
	return &Client{
		bot:      bot,
		commands: make(map[string]command),
		remna:    remna,
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
	callback := update.CallbackQuery
	log.Println("callback получен:", callback.Data, callback.From.ID)

	// Отвечаем на callback чтобы пропало отображение загрузки в телеграм
	ack := tgbotapi.NewCallback(callback.ID, "")
	if _, err := c.bot.AnswerCallbackQuery(ack); err != nil {
		log.Println("ошибка при ответе на callback", err)
	}

	// Парсим данные. Ожидаем формат "prefix_action_value"
	parts := strings.Split(callback.Data, "_")
	if len(parts) != 3 || parts[0] != "create" || parts[1] != "user" {
		log.Println("неверный формат callback data:", callback.Data)
		return
	}
	log.Println("callback успешно обработан:", callback.Data)

	months, err := strconv.Atoi(parts[2])
	if err != nil {
		log.Println("ошибка при парсинге количества месяцев:", err, parts[2])
		return
	}

	log.Println("месяцы подписки определены:", months)

	days := months * 30
	username := strconv.FormatInt(int64(callback.From.ID), 10)
	userUUID, err := c.remna.GetUUIDByUsername(username)
	if err != nil {
		if errors.Is(err, remnawave.ErrNotFound) {
			log.Println("пользователь не найден, создаем нового:", username)
			err = c.remna.CreateUser(username, days)
			if err != nil {
				log.Println("ошибка при создании пользователя:", err)
				return
			}
			log.Println("Пользователь", username, "создан на", days, "дней")
			msg := tgbotapi.NewEditMessageText(
				callback.Message.Chat.ID,
				callback.Message.MessageID,
				fmt.Sprintf("пользователь %s создан на %d дней", username, days),
			)
			if _, err := c.bot.Send(msg); err != nil {
				log.Println("ошибка при отправке сообщения:", err)
			}
		} else {
			log.Println("ошибка при получении UUID пользователя:", err)
			return
		}
	} else {
		log.Println("пользователь найден, продлеваем подписку:", username)
		err = c.remna.ExtendClientSubscription(userUUID, days)
		if err != nil {
			log.Println("ошибка при продлении подписки:", err)
			return
		}
		log.Println("Подписка для пользователя", username, "продлена на", days, "дней")
		msg := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			fmt.Sprintf("подписка для пользователя %s продлена на %d дней", username, days),
		)
		if _, err := c.bot.Send(msg); err != nil {
			log.Println("ошибка при отправке сообщения:", err)
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

// --------- команды ---------

// StartCommand обработчик команды /start
type StartCommand struct{}

func (s *StartCommand) Name() string {
	return "start"
}

func (s *StartCommand) Execute(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать, выберите тариф:")

	msg.ReplyMarkup = newTrafficKeyboard()

	_, err := bot.Send(msg)
	return err
}
