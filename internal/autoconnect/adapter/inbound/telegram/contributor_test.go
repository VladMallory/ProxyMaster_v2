package autoconnecttg

import (
	"testing"

	"github.com/VladMallory/ProxyMaster_v2/internal/autoconnect"
	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	"gopkg.in/telebot.v4"
)

type stubLinkBuilder struct {
	happ string
	incy string
}

func (s stubLinkBuilder) Build(provider, _ string) string {
	switch provider {
	case autoconnect.ProviderHapp:
		return s.happ
	case autoconnect.ProviderIncy:
		return s.incy
	default:
		return ""
	}
}

func TestRowsBuildHappAndIncyLinks(t *testing.T) {
	t.Parallel()

	happURL := "https://connect.example.com/happ?url=sub&sig=signature"
	incyURL := "https://connect.example.com/incy?url=sub&sig=signature"

	contributor := NewContributor(stubLinkBuilder{happ: happURL, incy: incyURL})
	menu := &telebot.ReplyMarkup{}

	rows := contributor.Rows(menu, platformtg.MenuContent{SubURL: "https://sub.example.com/u"})

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	if len(rows[0]) != 2 {
		t.Fatalf("apps row = %d buttons, want 2", len(rows[0]))
	}

	if got := rows[0][0].Text; got != "🚀 Happ" {
		t.Fatalf("happ text = %q, want 🚀 Happ", got)
	}

	if got := rows[0][0].URL; got != happURL {
		t.Fatalf("happ URL = %q, want %q", got, happURL)
	}

	if got := rows[0][1].Text; got != "🚀 Incy" {
		t.Fatalf("incy text = %q, want 🚀 Incy", got)
	}

	if got := rows[0][1].URL; got != incyURL {
		t.Fatalf("incy URL = %q, want %q", got, incyURL)
	}
}

func TestRowsHideIncyWhenBuilderReturnsEmpty(t *testing.T) {
	t.Parallel()

	happURL := "https://connect.example.com/happ?url=sub&sig=signature"
	contributor := NewContributor(stubLinkBuilder{happ: happURL})
	menu := &telebot.ReplyMarkup{}

	rows := contributor.Rows(menu, platformtg.MenuContent{SubURL: "https://sub.example.com/u"})

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	if len(rows[0]) != 1 {
		t.Fatalf("apps row = %d buttons, want 1", len(rows[0]))
	}

	if got := rows[0][0].Text; got != "🚀 Happ" {
		t.Fatalf("happ text = %q, want 🚀 Happ", got)
	}
}

func TestRowsFallbackToRawHappWithoutBuilder(t *testing.T) {
	t.Parallel()

	const raw = "https://sub.example.com/u"

	contributor := NewContributor(nil)
	menu := &telebot.ReplyMarkup{}

	rows := contributor.Rows(menu, platformtg.MenuContent{SubURL: raw})

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	if len(rows[0]) != 1 {
		t.Fatalf("apps row = %d buttons, want 1", len(rows[0]))
	}

	if got := rows[0][0].URL; got != raw {
		t.Fatalf("happ URL = %q, want raw %q", got, raw)
	}
}

func TestRowsWithoutSubscriptionReturnsNothing(t *testing.T) {
	t.Parallel()

	contributor := NewContributor(stubLinkBuilder{})
	menu := &telebot.ReplyMarkup{}

	rows := contributor.Rows(menu, platformtg.MenuContent{})

	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
}

func TestHandlers(t *testing.T) {
	t.Parallel()

	c := NewContributor(nil)

	if got := c.Handlers(); got != nil {
		t.Fatalf("ожидалось %v", got)
	}
}

func TestOrder(t *testing.T) {
	t.Parallel()

	c := NewContributor(nil)

	if got := c.Order(); got != 15 {
		t.Fatalf("ожидалось %d", got)
	}
}
