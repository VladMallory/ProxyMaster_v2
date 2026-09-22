package remnawave

import (
	"context"
	"strconv"

	"gopkg.in/telebot.v4"
)

type AdminNotifier struct {
	bot     *telebot.Bot
	adminID int64
}

func NewAdminNotifier(bot *telebot.Bot, adminID int64) *AdminNotifier {
	return &AdminNotifier{bot: bot, adminID: adminID}
}

func (n *AdminNotifier) Notify(ctx context.Context, err error, meta ErrorMeta) {
	_ = ctx

	if err == nil {
		return
	}

	if n.adminID == 0 {
		return
	}

	text := "🚨 " + meta.Op + ": " + err.Error() + "\n username: " + meta.Username

	_, _ = n.bot.Send(&telebot.Chat{ID: n.adminID}, text)
}

func ParseAdminID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)

	return id
}
