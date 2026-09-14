package telegram

import (
	"gopkg.in/telebot.v4"
)

// BuildStartMenu собирает итоговую клавиатуру /start из всех вкладчиков.
func (r *Registry) BuildStartMenu(content MenuContent) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	var rowsResult []telebot.Row

	// идём по отсортированным вкладчикам каждый делает append своих строк
	for _, c := range r.contributors {
		for _, r := range c.Rows(menu, content) {
			rowsResult = append(rowsResult, telebot.Row(r))
		}
	}

	// если вкладчиков нет вернётся пустое меню
	if len(rowsResult) > 0 {
		menu.Inline(rowsResult...)
	}

	return menu
}
