package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/distribution"
	"gopkg.in/telebot.v4"
)

const minInterval = 100 * time.Millisecond

type plainSender interface {
	Send(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error)
}

type Sender struct {
	bot   plainSender
	pace  time.Duration
	sleep func(time.Duration)
}

func NewSender(bot plainSender) *Sender {
	return &Sender{
		bot:   bot,
		pace:  minInterval,
		sleep: time.Sleep,
	}
}

func (s *Sender) Send(ctx context.Context, chatID int64, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.sleep(s.pace)

	if _, err := s.bot.Send(telebot.ChatID(chatID), text); err != nil {
		var tgErr *telebot.Error
		if errors.As(err, &tgErr) && tgErr.Code == http.StatusForbidden {
			return fmt.Errorf("%w: %s", distribution.ErrBlockedByUser, tgErr.Description)
		}

		return err
	}

	return nil
}
