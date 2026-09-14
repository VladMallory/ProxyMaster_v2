package telegramhandler

import (
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type PaymentContributor struct {
	handler *Handler
}

func NewPaymentContributor(h *Handler) *PaymentContributor {
	return &PaymentContributor{handler: h}
}

func (p *PaymentContributor) Order() int { return 20 }

func (p *PaymentContributor) Rows(
	menu *telebot.ReplyMarkup,
	_ telegram.MenuContent,
) [][]telebot.Btn {
	btnPay := menu.Data("💳 Оплатить", "pay_checkout")

	return [][]telebot.Btn{{btnPay}}
}

func (p *PaymentContributor) Handlers() map[string]telebot.HandlerFunc {
	return map[string]telebot.HandlerFunc{
		"pay_checkout": p.handler.handleCheckout,
		"pay_tariff":   p.handler.handleTariff,
	}
}
