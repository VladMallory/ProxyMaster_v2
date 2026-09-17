package telegramhandler

import (
	"context"
	"fmt"
	"strconv"

	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	"gopkg.in/telebot.v4"
)

type Handler struct {
	svc      *paymentsvc.Service
	notifier rememberer
}

type rememberer interface {
	Remember(userID string, msg telebot.Editable)
}

func NewHandler(svc *paymentsvc.Service, notifier rememberer) *Handler {
	return &Handler{
		svc:      svc,
		notifier: notifier,
	}
}

func (h *Handler) handleCheckout(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	tariffs := h.svc.Tariffs()
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, 4)
	row := make([]telebot.Btn, 0, 2)

	for i, t := range tariffs {
		text := fmt.Sprintf("💰%d₽ - %d месяца", t.PriceRub, t.Months)
		btn := menu.Data(text, "pay_tariff", strconv.Itoa(i))
		row = append(row, btn)

		if len(row) == 2 || i == len(tariffs)-1 {
			rows = append(rows, menu.Row(row...))
			row = nil
		}
	}

	btnBack := menu.Data("🏠В главное меню", "users_back")
	rows = append(rows, menu.Row(btnBack))

	menu.Inline(rows...)

	return c.Edit("Выберите сумму для пополнения баланса", menu)
}

func (h *Handler) handleTariff(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	idx, err := strconv.Atoi(c.Data())
	if err != nil || idx < 0 || idx >= len(h.svc.Tariffs()) {
		return c.Send("Неверный тариф, нажмите /start")
	}

	userID := strconv.FormatInt(c.Sender().ID, 10)

	h.notifier.Remember(userID, c.Message())

	payURL, err := h.svc.CreatePayment(context.Background(), userID, idx)
	if err != nil {
		return c.Send(err.Error())
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить", payURL)
	btnBack := menu.Data("🏠В главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}
