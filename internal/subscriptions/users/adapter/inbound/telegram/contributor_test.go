package telegram

import (
	"testing"

	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

func TestRowsDownloadAndSupport(t *testing.T) {
	t.Parallel()

	contributor := NewSubscriptionContributor(nil)
	menu := &telebot.ReplyMarkup{}

	rows := contributor.Rows(menu, platformtg.MenuContent{SubURL: "https://sub.example.com/u"})

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	if got := rows[0][0].Text; got != "📲 Скачать приложение" {
		t.Fatalf("download text = %q, want 📲 Скачать приложение", got)
	}
}
