package telegram

import (
	"github.com/VladMallory/ProxyMaster_v2/internal/delivery/transport/telegram"
)

type Sender struct {
	client *telegram.Client
}

func NewSender(client *telegram.Client) *Sender {
	return &Sender{client: client}
}

func (s *Sender) Send(chatID int64, text string) error {
	return s.client.SendMessage(chatID, text)
}
