package incy

import (
	"context"

	incylink "github.com/INCY-DEV/incy-link-encoder/go"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/inbound/http/page"
)

// NewHandler создаёт страницу автоподключения для Incy.
func NewHandler(links *autoconnect.Service, providerName string) *page.Handler {
	return page.NewHandler(links, deepLink(providerName))
}

// deepLink шифрует ссылку подписки в incy://crypt1/<payload> и передаёт имя,
// которое приложение покажет в списке подписок.
// Контекст не используется, шифрование локальное, без сетевых вызовов.
func deepLink(providerName string) page.DeepLinkBuilder {
	return func(_ context.Context, link autoconnect.Link) (string, error) {
		return incylink.EncryptLink(link.Subscription, providerName)
	}
}
