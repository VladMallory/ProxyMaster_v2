package telegram

import (
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type SubscriptionContributor struct {
	supportURL string
	handler    *Handler
}

// NewSubscriptionContributor регистрация в platform.Registry.
func NewSubscriptionContributor(supportURL string, handler *Handler) *SubscriptionContributor {
	return &SubscriptionContributor{
		supportURL: supportURL,
		handler:    handler,
	}
}

// Order вес в /start.
func (s *SubscriptionContributor) Order() int { return 10 }

// Rows адаптер делает append своих строк.
func (s *SubscriptionContributor) Rows(
	menu *telebot.ReplyMarkup,
	content platformtg.MenuContent,
) [][]telebot.Btn {
	btnDownload := menu.Data("📲 Скачать приложение", "users_download")
	btnURL := menu.URL("🚀 Подключиться", content.SubURL)
	btnSupport := menu.URL("🛟 Поддержка", s.supportURL)

	return [][]telebot.Btn{
		{btnDownload},
		{btnURL},
		{btnSupport},
	}
}

// Handlers все callback модуля подписок.
func (s *SubscriptionContributor) Handlers() map[string]telebot.HandlerFunc {
	return map[string]telebot.HandlerFunc{
		"users_download":       s.handler.handleDownload,
		"users_dl_ios":         s.handler.handleIOS,
		"users_dl_android":     s.handler.handleAndroid,
		"users_dl_linux":       s.handler.handleLinux,
		"users_dl_macos":       s.handler.handleMacOS,
		"users_dl_router":      s.handler.handleRouter,
		"users_back_platforms": s.handler.handleBackPlatforms,
	}
}
