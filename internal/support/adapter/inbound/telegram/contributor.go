package supportbtn

import (
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type SupportBtn struct {
	supportURL string
}

func NewSupportBtn(supportURL string) *SupportBtn {
	return &SupportBtn{
		supportURL: supportURL,
	}
}

// Order вес в /start.
func (s *SupportBtn) Order() int { return 20 }

func (s *SupportBtn) Rows(
	menu *telebot.ReplyMarkup,
	_ platformtg.MenuContent,
) [][]telebot.Btn {
	btnSupport := menu.URL("🛟 Поддержка", s.supportURL)

	return [][]telebot.Btn{
		{btnSupport},
	}
}

func (s *SupportBtn) Handlers() map[string]telebot.HandlerFunc {
	return nil
}
