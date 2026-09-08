package telegram

import "gopkg.in/telebot.v4"

// Button кнопка главного меню, зарегистрированная внешним адаптером.
type Button struct {
	Row int
	Btn telebot.Btn
}

// Registry общее хранилище кнопок главного меню.
// Не знает ни про подписки, ни про платежи — вообще ни про одну фичу.
type Registry struct {
	buttons []Button
}

func NewRegistry() *Registry {
	return &Registry{}
}

// Register добавляет кнопку в общее меню. Вызывается КАЖДЫМ адаптером самостоятельно.
func (r *Registry) Register(b Button) {
	r.buttons = append(r.buttons, b)
}

// Build собирает итоговые ряды кнопок вызывается тем, кто рисует /start.
func (r *Registry) Build(menu *telebot.ReplyMarkup) []telebot.Row {
	rows := make([]telebot.Row, 0, len(r.buttons))

	for _, b := range r.buttons {
		rows = append(rows, menu.Row(b.Btn))
	}

	return rows
}
