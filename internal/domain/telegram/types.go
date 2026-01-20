package telegram

import "sync"

// MessageSender — интерфейс, который должен реализовать "отправитель" (в нашем случае Telegram клиент).
// Это позволяет бизнес-логике не зависеть от конкретной библиотеки (tgbotapi).
type MessageSender interface {
	// SendMessage отправляет обычное текстовое сообщение в чат
	SendMessage(chatID int64, text string) error
	// ShowView отправляет сообщение с нужной клавиатурой
	// viewType: "tariffs", "payment", "main"
	// messageID: ID сообщения для редактирования (0 — отправить новое)
	ShowView(chatID int64, messageID int, viewType string, data string) error
}

// paymentPurpose описывает цель платежа
type paymentPurpose string

const (
	paymentPurposeAddDevice     paymentPurpose = "add_device"
	paymentPurposePrepayDevices paymentPurpose = "prepay_devices"
)

// paymentPurposeData хранит информацию о цели и сумме платежа
type paymentPurposeData struct {
	purpose paymentPurpose
	amount  int
}

// paymentPurposeByTransaction хранит сопоставление ID транзакции и цели платежа
var paymentPurposeByTransaction sync.Map

// activePaymentStatusWatchers хранит активные горутины проверки статуса платежей
var activePaymentStatusWatchers sync.Map
