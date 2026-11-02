package telegramhandler

import (
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

func TestRows(t *testing.T) {
	t.Parallel()

	c := NewPaymentContributor(nil)

	menu := telebot.ReplyMarkup{}

	got := c.Rows(&menu, telegram.MenuContent{SubURL: "https://example.com"})

	if len(got) != 1 {
		t.Fatalf("ожидалось 1, пришло %d", len(got))
	}
}

func TestOrder(t *testing.T) {
	t.Parallel()

	c := NewPaymentContributor(nil)

	got := c.Order()

	if got != 15 {
		t.Fatalf("пришло %d, ожидалось 15", got)
	}
}

func TestHandlers(t *testing.T) {
	t.Parallel()

	c := NewPaymentContributor(nil)

	got := c.Handlers()

	if len(got) != 2 {
		t.Fatalf("ожидалось 2, пришло %v", got)
	}

	if _, ok := got["pay_checkout"]; !ok {
		t.Fatalf("нету pay_checkout")
	}

	if _, ok := got["pay_tariff"]; !ok {
		t.Fatalf("нету pay_tariff")
	}
}
