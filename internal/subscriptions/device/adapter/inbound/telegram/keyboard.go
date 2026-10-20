package telegramdevice

import (
	"fmt"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"gopkg.in/telebot.v4"
)

func devicesMenu(devices []device.Device) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, len(devices)+1)
	for i, d := range devices {
		if !d.Bound() {
			continue
		}

		btnDel := menu.Data(
			fmt.Sprintf("%d удалить", i+1),
			"devices_del",
			strconv.FormatInt(d.ID, 10),
		)
		rows = append(rows, menu.Row(btnDel))
	}

	rows = append(rows, menu.Row(menu.Data("🏠 В главное меню", "users_back")))

	menu.Inline(rows...)

	return menu
}
