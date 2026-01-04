// Package telegram реализация работы с telegram api
package telegram

import (
	"ProxyMaster_v2/pkg/logger"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Handler - интерфейс для обработки входящих обновлений (сообщений, кнопок)
type Handler interface {
	Handle(update tgbotapi.Update, bot *tgbotapi.BotAPI) error
}

// Client - обертка над API телеграма
type Client struct {
	// Само апи телеграмма
	bot *tgbotapi.BotAPI
	// Единый обработчик логики
	handler Handler

	// Чтобы логировать все действия
	logger logger.Logger
}

// NewClient - конструктор
func NewClient(bot *tgbotapi.BotAPI, handler Handler, logger logger.Logger) *Client {
	logger.Info("Создан экземпляр TelegramClient")

	return &Client{
		bot:     bot,
		handler: handler,
		logger:  logger,
	}
}

// Run - запуск цикла получения сообщений
func (c *Client) Run() {
	// получаем канал обновлений
	updates, err := c.initUpdatesChannel()
	if err != nil {
		c.logger.Error("ошибка получения канала обновлений",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "method", Value: "initUpdatesChannel"},
		)
		return
	}

	// читаем сообщения из канала в бесконечном цикле
	for update := range updates {
		// Передаем обновление в handler
		// Запускаем в горутине, чтобы обработка одного сообщения не тормозила остальные
		go func(u tgbotapi.Update) {
			if err := c.handler.Handle(u, c.bot); err != nil {
				slog.Error("ошибка обработки обновления", "error", err)
			}
		}(update)
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

	updates, err := c.bot.GetUpdatesChan(u)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения канала обновлений: %w", err)
	}
	return updates, nil
}
