package telegramdevice

import (
	"context"
	"fmt"
	"html"
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
	var b strings.Builder

	b.WriteString("📱 <b>Ваши устройства</b>\n\n")

	if len(devices) == 0 {
		b.WriteString("Пока нет ни одного устройства\n\n")
		b.WriteString("<i>Появится тут после подключения</i>")

		return b.String()
	}

	// Занятые места
	var bound int

	for i, d := range devices {
		if d.Bound() {
			bound++

			fmt.Fprintf(
				&b,
				"🟢 <b>%d. %s</b>\n\n",
				i+1,
				html.EscapeString(devicesTitle(d)),
			)

			continue
		}

		fmt.Fprintf(&b, "⚪ <b>%d. Свободное место</b>\n\n", i+1)
	}

	b.WriteString("───────────\n")

	fmt.Fprintf(&b, "Занято: <b>%d</b> из <b>%d</b>", bound, len(devices))

	return b.String()
}

// devicesTitle решает как показать место.
func devicesTitle(d device.Device) string {
	if d.Name != "" {
		return d.Name
	}

	return "Устройство"
}
