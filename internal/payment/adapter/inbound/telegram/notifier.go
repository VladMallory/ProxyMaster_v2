package telegramhandler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"gopkg.in/telebot.v4"
)

type mainMenuBuilder interface {
	BuildMainMenu(ctx context.Context, userID, name string) (string, *telebot.ReplyMarkup)
}

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
		fmt.Println("NotifySuccess удалил ключ")
		delete(n.msgs, userID)
	}

	n.mu.Unlock()
	if !ok {
		return
	}

	fmt.Println("Успешная оплата, ", userID, months)

	_, err := n.bot.Edit(msg, "Оплата прошла успешно ")
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
		fmt.Println("NotifyTimeout удалил ключ")
		delete(n.msgs, userID)
	}

	n.mu.Unlock()
	if !ok {
		return
	}

	fmt.Println("NotifyTimeout: ", userID)

	_, err := n.bot.Edit(msg, "da")
	if err != nil {
		return
	}
}
