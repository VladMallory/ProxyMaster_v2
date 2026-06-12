// Package telegram содержит реализацию взаимодействия с Telegram API (tgbotapi).
// Это единственный пакет в проекте, который импортирует библиотеку tgbotapi.
//
//nolint:funlen,cyclop
package telegram

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const parseModeHTML = "HTML"

// Client — тонкая обертка над tgbotapi.BotAPI.
// Отвечает ТОЛЬКО за отправку/получение данных через Telegram API.
type Client struct {
	api *tgbotapi.BotAPI
}

// New создает клиента Telegram API.
func New(token string, socks5 SOCKS5Config) (*Client, error) {
	httpClient := createHTTPClient(socks5)

	api, err := tgbotapi.NewBotAPIWithClient(token, httpClient)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Telegram API: %w", err)
	}

	return &Client{api: api}, nil
}

// Start запускает бесконечный цикл long-polling.
// Для каждого обновления вызывает handler с упрощенной моделью Message.
func (c *Client) Start(handler func(Message)) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := c.api.GetUpdatesChan(u)
	if err != nil {
		return
	}

	for update := range updates {
		if update.CallbackQuery != nil {
			msg := Message{
				ChatID:     update.CallbackQuery.Message.Chat.ID,
				MessageID:  update.CallbackQuery.Message.MessageID,
				Data:       update.CallbackQuery.Data,
				FirstName:  update.CallbackQuery.From.FirstName,
				IsCallback: true,
			}
			handler(msg)

			cd := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			_, _ = c.api.AnswerCallbackQuery(cd)

			continue
		}

		if update.Message != nil {
			firstName := ""
			telegramUsername := ""
			if update.Message.From != nil {
				firstName = update.Message.From.FirstName
				telegramUsername = update.Message.From.UserName
			}

			handler(Message{
				ChatID:           update.Message.Chat.ID,
				Text:             update.Message.Text,
				FirstName:        firstName,
				TelegramUsername: telegramUsername,
			})
		}
	}
}

// SendMessage отправляет текстовое сообщение с inline-клавиатурой.
func (c *Client) SendMessage(chatID int64, text string, keyboard InlineKeyboard) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseModeHTML
	msg.ReplyMarkup = convertKeyboard(keyboard)

	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения: %w", err)
	}

	return nil
}

// EditMessage редактирует существующее сообщение, заменяя текст и клавиатуру.
func (c *Client) EditMessage(
	chatID int64,
	messageID int,
	text string,
	keyboard InlineKeyboard,
) error {
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	msg.ParseMode = parseModeHTML
	markup := convertKeyboard(keyboard)
	msg.ReplyMarkup = &markup

	_, err := c.api.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка редактирования сообщения: %w", err)
	}

	return nil
}

// SetCommands регистрирует список команд бота (команды по / и в меню ≡).
// Вызывается один раз при старте бота.
func (c *Client) SetCommands(commandsJSON string) error {
	parameter := url.Values{}
	parameter.Set("commands", commandsJSON)

	_, err := c.api.MakeRequest("setMyCommands", parameter)
	if err != nil {
		return err
	}

	return nil
}

// SetMenuButton включает кнопку меню (три полоски) и привязывает её к списку команд.
// Когда пользователь нажимает ≡ — видит команды бота.

func (c *Client) SetMenuButton() error {
	parameter := url.Values{}
	parameter.Set("menu_button", `{"type":"commands"}`)

	_, err := c.api.MakeRequest("setChatMenuButton", parameter)
	if err != nil {
		return err
	}

	return nil
}

// SetupCommandsAndMenu удобная обёртка, регистрирует /start и включает кнопку меню.
func (c *Client) SetupCommandsAndMenu() error {
	// Регистрирует /start как единственную команду
	// JSON-формат: [{"command":"start","description":"Начать работу"}]
	if err := c.SetCommands(`[{"command":"start","description":"Запуск"}]`); err != nil {
		return err
	}

	// Включаем кнопку меню (три полоски) — теперь при нажатии ≡ покажет команды
	if err := c.SetMenuButton(); err != nil {
		return err
	}

	return nil
}

// convertKeyboard преобразует абстрактную InlineKeyboard в tgbotapi.InlineKeyboardMarkup.
func convertKeyboard(k InlineKeyboard) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, len(k))
	for i, row := range k {
		btns := make([]tgbotapi.InlineKeyboardButton, len(row))
		for j, btn := range row {
			if btn.URL != "" {
				btns[j] = tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL)
			} else {
				btns[j] = tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data)
			}
		}
		rows[i] = tgbotapi.NewInlineKeyboardRow(btns...)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// createHTTPClient создает HTTP-клиент с опциональным SOCKS5 прокси.
func createHTTPClient(socks5 SOCKS5Config) *http.Client {
	if socks5.Host == "" || socks5.Port == "" {
		return &http.Client{}
	}

	proxyURL := &url.URL{
		Scheme: "socks5",
		User:   url.UserPassword(socks5.Username, socks5.Password),
		Host:   net.JoinHostPort(socks5.Host, socks5.Port),
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
	}
}
