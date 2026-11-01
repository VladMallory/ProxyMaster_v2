package happ

import (
	"context"
	"log"
	"net/url"
	"strings"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect/adapter/inbound/http/page"
)

// Encrypter превращает ссылку подписки в зашифрованный deep link Happ.
type Encrypter interface {
	Encrypt(ctx context.Context, subscriptionURL string) (string, error)
}

// NewHandler создаёт страницу автоподключения для Happ.
func NewHandler(links *autoconnect.Service, enc Encrypter) *page.Handler {
	return page.NewHandler(links, deepLink(enc))
}

// deepLink отдаёт happ://crypt5/<payload>. Если шифрование недоступно,
// возвращает обычную ссылку: подключение важнее скрытности.
func deepLink(enc Encrypter) page.DeepLinkBuilder {
	return func(ctx context.Context, link autoconnect.Link) (string, error) {
		encrypted, err := enc.Encrypt(ctx, link.Subscription)
		if err != nil {
			log.Printf("не зашифровал ссылку happ: %v", err)

			return addLink(link.Subscription), nil
		}

		return encrypted, nil
	}
}

// addLink собирает happ://add/<subscription>.
// Вложенную ссылку кодируем как encodeURIComponent в браузере: иначе Happ
// обрежет query-параметры подписки на первом же `&`.
func addLink(subscription string) string {
	return "happ://add/" + encodeURIComponent(subscription)
}

func encodeURIComponent(raw string) string {
	// QueryEscape кодирует пробел как `+`, а в пути deep link нужен `%20`.
	return strings.ReplaceAll(url.QueryEscape(raw), "+", "%20")
}
