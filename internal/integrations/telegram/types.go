// Package telegram содержит абстрактные типы для взаимодействия с Telegram API.
// Не имеет зависимости от конкретной библиотеки (tgbotapi).
// Business logic (в transport/telegram) использует эти типы вместо tgbotapi.
package telegram

// ButtonData — data for one inline keyboard button.
type ButtonData struct {
	Text string
	Data string // callback data
	URL  string // optional URL
}

// InlineKeyboard — матрица кнопок (строки × столбцы).
type InlineKeyboard [][]ButtonData

// NewButton creates button with callback data.
func NewButton(text, data string) ButtonData {
	return ButtonData{Text: text, Data: data}
}

// NewURLButton creates button with external URL.
func NewURLButton(text, url string) ButtonData {
	return ButtonData{Text: text, URL: url}
}

// Message — упрощенная модель входящего обновления от Telegram.
type Message struct {
	ChatID           int64
	MessageID        int // 0 если это обычное сообщение, >0 если callback
	Text             string
	Data             string // callback data
	FirstName        string
	TelegramUsername string
	IsCallback       bool
}

// SOCKS5Config — опциональная конфигурация прокси для Telegram API.
type SOCKS5Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

// BotAPI — интерфейс низкоуровневого Telegram API.
// Бизнес-логика (internal/delivery/transport/telegram) зависит от этого интерфейса,
// а не от конкретной реализации.
type BotAPI interface {
	SendMessage(chatID int64, text string, keyboard InlineKeyboard) error
	EditMessage(chatID int64, messageID int, text string, keyboard InlineKeyboard) error
	Start(handler func(Message))
	SetupCommandsAndMenu() error
}
