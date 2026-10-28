package telegram

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/VladMallory/ProxyMaster_v2/internal/distribution"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

const Command = "/distribution"

type broadcaster interface {
	Broadcast(ctx context.Context, text string) (distribution.Report, error)
}

type sender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

type Handler struct {
	svc     broadcaster
	sender  sender
	adminID int64
	log     *zap.Logger
}

func NewHandler(
	svc broadcaster,
	sender sender,
	adminID int64,
	log *zap.Logger,
) *Handler {
	return &Handler{
		svc:     svc,
		sender:  sender,
		adminID: adminID,
		log:     log,
	}
}

func (h *Handler) Handle(c telebot.Context) error {
	from := c.Sender()
	if from == nil || from.ID != h.adminID {
		h.log.Warn("рассылка: попытка без прав")

		return c.Send("У вас нет прав для этой команды.")
	}

	text := messageText(c.Text())
	if text == "" {
		return c.Send("Напиши текст сообщения: /distribution Всем привет")
	}

	go h.run(text)

	return c.Send("🚀 Рассылка запущена, пришлю итог.")
}

func (h *Handler) run(text string) {
	report, err := h.svc.Broadcast(context.Background(), text)
	if err != nil {
		h.log.Error("рассылка: прогон не состоялся", zap.Error(err))
		h.notify(fmt.Sprintf("❌ Рассылка не пошла: %v", err))

		return
	}

	h.log.Info("рассылка: прогон завершён",
		zap.Int("total", report.Total),
		zap.Int("sent", report.Sent),
		zap.Int("blocked", report.Blocked),
		zap.Int("failed", report.Failed))

	h.notify(fmt.Sprintf(
		"✅ Рассылка завершена.\nПолучателей: %d\nДоставлено: %d\nНе смогли написать: %d\nОшибок: %d",
		report.Total,
		report.Sent,
		report.Blocked,
		report.Failed,
	))
}

func (h *Handler) notify(text string) {
	if err := h.sender.Send(context.Background(), h.adminID, text); err != nil {
		h.log.Error("рассылка: не удалось отправить итог админу", zap.Error(err))
	}
}

func messageText(raw string) string {
	cut := strings.IndexFunc(raw, unicode.IsSpace)
	if cut < 0 {
		return ""
	}

	return strings.TrimSpace(raw[cut:])
}
