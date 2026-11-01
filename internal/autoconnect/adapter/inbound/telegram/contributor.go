package autoconnecttg

import (
	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

// LinkBuilder собирает подписанные ссылки на страницы автоподключения.
type LinkBuilder interface {
	Build(provider, subscriptionURL string) string
}

type Contributor struct {
	links LinkBuilder
}

func NewContributor(links LinkBuilder) *Contributor {
	return &Contributor{links: links}
}

func (c *Contributor) Order() int { return 15 }

func (c *Contributor) Rows(
	menu *telebot.ReplyMarkup,
	content platformtg.MenuContent,
) [][]telebot.Btn {
	var apps []telebot.Btn

	if happURL := c.happURL(content.SubURL); happURL != "" {
		apps = append(apps, menu.URL("🚀 Happ", happURL))
	}

	if incyURL := c.incyURL(content.SubURL); incyURL != "" {
		apps = append(apps, menu.URL("🚀 Incy", incyURL))
	}

	if len(apps) == 0 {
		return nil
	}

	return [][]telebot.Btn{apps}
}

// Handlers у фичи нет callback-кнопок, только URL.
func (c *Contributor) Handlers() map[string]telebot.HandlerFunc {
	return nil
}

// happURL возвращает подписанную ссылку на страницу Happ.
// Если автоподключение выключено, остаётся сырая ссылка подписки.
func (c *Contributor) happURL(raw string) string {
	if c.links == nil {
		return raw
	}

	if url := c.links.Build(autoconnect.ProviderHapp, raw); url != "" {
		return url
	}

	return raw
}

// incyURL возвращает подписанную ссылку на страницу Incy.
// Пустая строка означает, что кнопку Incy показывать не нужно.
func (c *Contributor) incyURL(raw string) string {
	if c.links == nil {
		return ""
	}

	return c.links.Build(autoconnect.ProviderIncy, raw)
}
