package devicetg

import (
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type Contributor struct {
	handler *Handler
}

func NewContributor(h *Handler) *Contributor {
	return &Contributor{handler: h}
}

func (d *Contributor) Order() int { return 15 }

func (d *Contributor) Rows(menu *telebot.ReplyMarkup, _ platformtg.MenuContent) [][]telebot.Btn {
	return [][]telebot.Btn{
		{menu.Data("📱 Ваши устройства", "devices_list")},
	}
}

func (d *Contributor) Handlers() map[string]telebot.HandlerFunc {
	return map[string]telebot.HandlerFunc{
		"devices_list":       d.handler.handleList,
		"devices_del":        d.handler.handleDelete,
		"devices_extend_ask": d.handler.handleExtendAsk,
		"devices_extend":     d.handler.handleExtend,
		"devices_buy":        d.handler.handleBuy,
	}
}
