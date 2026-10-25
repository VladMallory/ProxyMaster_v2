package telegramdevice

import (
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type Contributor struct {
	handler *HandlerTelegram
}

func NewContributor(handler *HandlerTelegram) *Contributor {
	return &Contributor{
		handler: handler,
	}
}

func (c *Contributor) Order() int { return 15 }

func (c *Contributor) Rows(
	menu *telebot.ReplyMarkup,
	_ telegram.MenuContent,
) [][]telebot.Btn {
	btnDevice := menu.Data("📱 Ваши устройства", "device_list")

	return [][]telebot.Btn{{btnDevice}}
}

func (c *Contributor) Handlers() map[string]telebot.HandlerFunc {
	return map[string]telebot.HandlerFunc{
		"device_list":     c.handler.handleList,
		"device_open":     c.handler.handleOpen,
		"devices_del":     c.handler.handleDelete,
		"device_extend":   c.handler.handleExtend,
		"device_term":     c.handler.handleTerm,
		"device_add":      c.handler.handleAdd,
		"device_add_term": c.handler.handleAddTerm,
	}
}
