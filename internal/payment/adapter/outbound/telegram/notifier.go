// internal/payment/adapter/outbound/telegram/notifier.go
package telegram

import (
	"context"
	"fmt"
	"strconv"

	"gopkg.in/telebot.v4"
)

type Notifier struct {
	bot *telebot.Bot
}

func NewNotifier(bot *telebot.Bot) *Notifier {
	return &Notifier{bot: bot}
}

func (n *Notifier) OnPaymentConfirmed(ctx context.Context, userID string, amount int) error {
	chatID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	_, err = n.bot.Send(
		&telebot.User{ID: chatID},
		fmt.Sprintf("✅ Подписка продлена на %d ₽", amount),
	)

	return err
}
