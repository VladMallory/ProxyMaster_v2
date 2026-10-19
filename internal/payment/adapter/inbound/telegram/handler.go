package telegramhandler

import (
	"context"
	"fmt"
	"strconv"

	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	"gopkg.in/telebot.v4"
)

type subscriptionExtender interface {
	ExtendSubscription(ctx context.Context, username string, month int) error
}

type Handler struct {
	svc      *paymentsvc.Service
	notifier rememberer
	subs     subscriptionExtender
}

type rememberer interface {
	Remember(userID string, msg telebot.Editable)
}

func NewHandler(svc *paymentsvc.Service, notifier rememberer, subs subscriptionExtender) *Handler {
	return &Handler{
		svc:      svc,
		notifier: notifier,
		subs:     subs,
	}
}

func (h *Handler) handleCheckout(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	menu := &telebot.ReplyMarkup{}
	tariffs := h.svc.Tariffs()

	rows := make([]telebot.Row, 0, 4)
	row := make([]telebot.Btn, 0, 2)

	for i, t := range tariffs {
		text := fmt.Sprintf("💰%d₽ - %d мес.", t.PriceRub, t.Months)
		btn := menu.Data(text, "pay_tariff", strconv.Itoa(i))
		row = append(row, btn)

		if len(row) == 2 || i == len(tariffs)-1 {
			rows = append(rows, menu.Row(row...))
			row = nil
		}
	}

	rows = append(rows, menu.Row(menu.Data("🏠 В главное меню", "users_back")))
	menu.Inline(rows...)

	return c.Edit("Выберите срок подписки:", menu)
}

func (h *Handler) handleTariff(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	idx, err := strconv.Atoi(c.Data())
	if err != nil || idx < 0 || idx >= len(h.svc.Tariffs()) {
		return c.Send("Неверный тариф, нажмите /start")
	}

	tariff := h.svc.Tariffs()[idx]
	username := strconv.FormatInt(c.Sender().ID, 10)

	h.notifier.Remember(username, c.Message())

	onPaid := func(ctx context.Context, _ string) error {
		return h.subs.ExtendSubscription(ctx, username, tariff.Months)
	}

	payURL, err := h.svc.CreatePayment(context.Background(), username, tariff.PriceRub, onPaid)
	if err != nil {
		return c.Send(err.Error())
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить", payURL)
	btnBack := menu.Data("🏠 В главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}
