package page

import (
	"context"
	"html/template"
	"log"
	"net/http"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
)

// Opener расшифровывает токен из ссылки и возвращает адрес подписки.
type Opener interface {
	Open(token string) (string, bool)
}

// DeepLinkBuilder собирает deep link приложения из ссылки подписки.
// Контекст нужен адаптерам, которые ходят во внешние сервисы.
type DeepLinkBuilder func(ctx context.Context, link autoconnect.Link) (string, error)

// connectPage пустая страница, которая сразу передаёт подписку в приложение.
// html/template сам экранирует URL как JS-строку, поэтому кавычки и `</script>`
// в ссылке не сломают разметку.
const connectPage = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Подключение</title>
<style>html,body{margin:0;min-height:100%;background:#000}</style>
</head>
<body>
<script>
(function () {
    window.location.href = {{.DeepLink}};
})();
</script>
</body>
</html>`

type pageData struct {
	DeepLink string
}

// Handler отдаёт страницу автоподключения.
type Handler struct {
	opener Opener
	build  DeepLinkBuilder
	page   *template.Template
}

func NewHandler(opener Opener, build DeepLinkBuilder) *Handler {
	return &Handler{
		opener: opener,
		build:  build,
		page:   template.Must(template.New("connect").Parse(connectPage)),
	}
}

func (h *Handler) Register(mux *http.ServeMux, paths ...string) {
	for _, path := range paths {
		mux.Handle(path, h)
	}
}

// ServeHTTP расшифровывает токен и отдаёт страницу с deep link приложения.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	subscription, ok := h.opener.Open(r.URL.Query().Get("t"))
	if !ok {
		http.Error(w, "invalid link", http.StatusBadRequest)

		return
	}

	deepLink, err := h.build(r.Context(), autoconnect.Link{Subscription: subscription})
	if err != nil {
		log.Printf("autoconnect: собрать deep link: %v", err)
		http.Error(w, "cannot build link", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := h.page.Execute(w, pageData{DeepLink: deepLink}); err != nil {
		log.Printf("autoconnect: рендер страницы: %v", err)
	}
}
