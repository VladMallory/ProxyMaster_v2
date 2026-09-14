package telegram

import (
	"sort"

	"gopkg.in/telebot.v4"
)

// MenuContent контекст для построения кнопок /start.
// Нужен чтобы URL кнопка "Подключиться" могла подставить персональный URL.
type MenuContent struct {
	SubURL string
}

type MenuContributor interface {
	// Order задаёт порядок в /start. Сортирует platform.
	Order() int
	// Rows адаптер делает append своих строк в общее меню.
	// menu создаёт platform, адаптер только вызывает menu.Data / menu.URL.
	Rows(menu *telebot.ReplyMarkup, content MenuContent) [][]telebot.Btn
	// Handlers все callback-и модуля. Unique с префиксом (users_, pay_) чтобы не было коллизий.
	Handlers() map[string]telebot.HandlerFunc
}

// Registry собирает всех вкладчиков и строит итоговое меню.
type Registry struct {
	contributors []MenuContributor
}

// Register добавляет вкладчика и сортирует по Order.
// Порядок вывода контролирует platform, адаптер лишь заявляет свой вес.
func (r *Registry) Register(c MenuContributor) {
	r.contributors = append(r.contributors, c)
	sort.Slice(r.contributors, func(i, j int) bool {
		return r.contributors[i].Order() < r.contributors[j].Order()
	})
}

// contributorsSnapshot безопасный доступ для Setup без экспорта поля.
func (r *Registry) contributorsSnapshot() []MenuContributor {
	out := make([]MenuContributor, len(r.contributors))
	copy(out, r.contributors)

	return out
}
