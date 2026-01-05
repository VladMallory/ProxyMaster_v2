// Package telegram содержит бизнес логику бота и интерфейсы
// для взаимодействия. Определяется как себя будут вести команды и что выполнять
package telegram

// MessageSender — интерфейс, который должен реализовать "отправитель" (в нашем случае Telegram клиент).
// Это позволяет бизнес-логике не зависеть от конкретной библиотеки (tgbotapi).
type MessageSender interface {
	// SendMessage отправляет обычное текстовое сообщение в чат
	SendMessage(chatID int64, text string) error
	// ShowView отправляет сообщение с нужной клавиатурой
	// viewType: "tariffs", "payment", "main"
	// messageID: ID сообщения для редактирования (0 — отправить новое)
	ShowView(chatID int64, messageID int, viewType string) error
}

// ProcessCallback обрабатывает нажатия на инлайн-кнопки (которые под сообщениями).
// sender: кто будет отправлять ответ (наш telegram клиент)
// chatID: ID чата, куда отправлять ответ
// messageID: ID сообщения, которое нужно отредактировать
// data: скрытые данные, зашитые в кнопку (например, "btn_balance")
func ProcessCallback(sender MessageSender, chatID int64, messageID int, data string) error {
	switch data {
	case "btn_balance":
		// Показываем тарифы (редактируем текущее сообщение)
		return sender.ShowView(chatID, messageID, "tariffs")
	case "btn_tariff_1", "btn_tariff_3":
		// Если выбрали тариф — предлагаем методы оплаты
		return sender.ShowView(chatID, messageID, "payment")
	case "btn_pay_card", "btn_pay_crypto":
		// Заглушка для оплаты
		return sender.SendMessage(chatID, "Создаю ссылку на оплату... (тут будет интеграция с платежкой)")
	case "btn_connect":
		// Заглушка для кнопки подключения
		return sender.SendMessage(chatID, "Функция подключения пока в разработке")
	case "btn_info":
		// Информация о боте
		return sender.SendMessage(chatID, "В разработке")
	case "btn_back":
		// Кнопка "Назад" возвращает в главное меню
		return sender.ShowView(chatID, messageID, "main")
	default:
		return sender.SendMessage(chatID, "Неизвестная команда")
	}
}

// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(sender MessageSender, chatID int64, command string) error {
	var replyText string
	switch command {
	case "/start":
		replyText = "Добро пожаловать! Я помогу вам управлять подписками."
	default:
		replyText = "Неизвестная команда. Пожалуйста, используйте /start."
	}

	// Отправляем ответ
	return sender.SendMessage(chatID, replyText)
}
