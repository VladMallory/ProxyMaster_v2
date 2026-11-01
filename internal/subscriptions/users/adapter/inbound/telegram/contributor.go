package telegram

import (
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type SubscriptionContributor struct {
	handler *Handler
}

func NewSubscriptionContributor(
	// supportURL string,
	handler *Handler,
) *SubscriptionContributor {
	return &SubscriptionContributor{
		// supportURL: supportURL,
		handler: handler,
	}
}

func (s *SubscriptionContributor) Order() int { return 10 }

func (s *SubscriptionContributor) Rows(
	menu *telebot.ReplyMarkup,
	_ platformtg.MenuContent,
) [][]telebot.Btn {
	btnDownload := menu.Data("📲 Скачать приложение", "users_download")

	return [][]telebot.Btn{
		{btnDownload},
	}
}

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
