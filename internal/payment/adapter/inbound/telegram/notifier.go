package telegramhandler

import (
	"log/slog"
	"sync"

	"gopkg.in/telebot.v4"
)

type Notifier struct {
	bot  *telebot.Bot
	mu   sync.Mutex
	msgs map[string]telebot.Editable
}

func NewNotifier(bot *telebot.Bot) *Notifier {
	return &Notifier{
		bot:  bot,
		msgs: make(map[string]telebot.Editable),
	}
}

func (n *Notifier) NotifySuccess(userID string, months int) {
	n.mu.Lock()

	msg, ok := n.msgs[userID]
	if ok {
		delete(n.msgs, userID)
	}

	n.mu.Unlock()
	if !ok {
		return
	}

	_, err := n.bot.Edit(msg, "✅Оплата прошла успешно ",
		backMenu(), telebot.ModeHTML)
	if err != nil {
		slog.Error("не отправился success-нотифай", "user", userID, "err", err)
	}
}

func (n *Notifier) Remember(userID string, msg telebot.Editable) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.msgs[userID] = msg
}

func (n *Notifier) NotifyTimeout(userID string) {
	n.mu.Lock()

	msg, ok := n.msgs[userID]
	if ok {
		delete(n.msgs, userID)
	}

	n.mu.Unlock()
	if !ok {
		return
	}

	_, err := n.bot.Edit(msg, "⏳Время оплаты истекло.", backMenu(), telebot.ModeHTML)
	if err != nil {
		return
	}
}

func backMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	btnBack := menu.Data("🏠 В главное меню", "users_back")
	menu.Inline(menu.Row(btnBack))

	return menu
}
