package telegramdevice

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"gopkg.in/telebot.v4"
)

type deviceUseCase interface {
	ListDevices(ctx context.Context, username string) ([]device.Device, error)
}

type HandlerTelegram struct {
	useCase deviceUseCase
}

func NewHandlerTelegram(useCase deviceUseCase) *HandlerTelegram {
	return &HandlerTelegram{useCase: useCase}
}

// handleList оркестор для рендера текста и inaly кнопок.
func (h *HandlerTelegram) handleList(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	devices, err := h.useCase.ListDevices(context.Background(), username)
	if err != nil {
		return err
	}

	return c.Edit(devicesText(devices), devicesMenu(devices), telebot.ModeHTML)
}

// devicesText рендер текста для "Ваши устройства".
func devicesText(devices []device.Device) string {
	if len(devices) == 0 {
		return "📱 Ваши устройства\n\nПока пусто."
	}

	var b strings.Builder
	b.WriteString("📱 Ваши устройства\n\n")

	// Проверяем какие есть
	for i, d := range devices {
		fmt.Fprintf(&b, "%d: %s\n", i+1, devicesTitle(d))
	}

	return b.String()
}

// devicesTitle решает как показать место.
func devicesTitle(d device.Device) string {
	if !d.Bound() {
		return "Свободное место"
	}

	if d.Name != "" {
		return d.Name
	}

	return d.HWID
}
