package keyboard

import (
	"testing"

	platformtg "github.com/VladMallory/ProxyMaster_v2/internal/platform/telegram"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"github.com/stretchr/testify/require"
	"gopkg.in/telebot.v4"
)

// newTestKeyboard собирает Keyboard с тестовой ссылкой на поддержку и пустым реестром.
func newTestKeyboard() *Keyboard {
	return New("https://support.example", platformtg.NewRegistry())
}

func TestKeyboard_Start(t *testing.T) {
	t.Parallel()

	registry := platformtg.NewRegistry()
	registry.Register(platformtg.Button{
		Row: 2,
		Btn: telebot.Btn{Unique: "payment_menu", Text: "💳 Продлить подписку"},
	})

	kb := New("https://support.example", registry)
	menu := kb.Start(subdomain.User{URL: "https://sub.example"})

	// Четыре ряда: скачать / подключиться / продлить(из реестра) / поддержка.
	require.Len(t, menu.InlineKeyboard, 4)

	require.Len(t, menu.InlineKeyboard[0], 1)
	require.Equal(t, "users_download", menu.InlineKeyboard[0][0].Unique)

	require.Len(t, menu.InlineKeyboard[1], 1)
	require.Equal(t, "🚀 Подключиться", menu.InlineKeyboard[1][0].Text)
	require.Equal(t, "https://sub.example", menu.InlineKeyboard[1][0].URL)

	require.Len(t, menu.InlineKeyboard[2], 1)
	require.Equal(t, "payment_menu", menu.InlineKeyboard[2][0].Unique)

	require.Len(t, menu.InlineKeyboard[3], 1)
	require.Equal(t, "🛟 Поддержка", menu.InlineKeyboard[3][0].Text)
	require.Equal(t, "https://support.example", menu.InlineKeyboard[3][0].URL)
}

func TestKeyboard_DownloadApps(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().DownloadApps()

	require.Len(t, menu.InlineKeyboard, 4)

	// Ряд платформ: iOS и Android.
	require.Len(t, menu.InlineKeyboard[0], 2)
	require.Equal(t, "users_dl_ios", menu.InlineKeyboard[0][0].Unique)
	require.Equal(t, "users_dl_android", menu.InlineKeyboard[0][1].Unique)

	// Второй ряд: Linux, Windows (URL), macOS.
	require.Len(t, menu.InlineKeyboard[1], 3)
	require.Equal(t, "users_dl_linux", menu.InlineKeyboard[1][0].Unique)
	require.NotEmpty(t, menu.InlineKeyboard[1][1].URL)
	require.Equal(t, "users_dl_macos", menu.InlineKeyboard[1][2].Unique)

	// Третий ряд: роутер.
	require.Len(t, menu.InlineKeyboard[2], 1)
	require.Equal(t, "users_dl_router", menu.InlineKeyboard[2][0].Unique)

	// Последний ряд: возврат в главное меню.
	require.Len(t, menu.InlineKeyboard[3], 1)
	require.Equal(t, "users_back", menu.InlineKeyboard[3][0].Unique)
}

func TestKeyboard_IOS(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().IOS()

	require.Len(t, menu.InlineKeyboard, 3)
	require.Len(t, menu.InlineKeyboard[0], 1)
	require.NotEmpty(t, menu.InlineKeyboard[0][0].URL)
	require.Len(t, menu.InlineKeyboard[1], 1)
	require.NotEmpty(t, menu.InlineKeyboard[1][0].URL)

	// Кнопка возврата к списку платформ.
	require.Len(t, menu.InlineKeyboard[2], 1)
	require.Equal(t, "users_back_platforms", menu.InlineKeyboard[2][0].Unique)
}

func TestKeyboard_Android(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().Android()

	require.Len(t, menu.InlineKeyboard, 2)
	require.Len(t, menu.InlineKeyboard[0], 2)
	require.NotEmpty(t, menu.InlineKeyboard[0][0].URL)
	require.NotEmpty(t, menu.InlineKeyboard[0][1].URL)

	require.Len(t, menu.InlineKeyboard[1], 1)
	require.Equal(t, "users_back_platforms", menu.InlineKeyboard[1][0].Unique)
}

func TestKeyboard_Linux(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().Linux()

	require.Len(t, menu.InlineKeyboard, 3)
	require.Len(t, menu.InlineKeyboard[0], 2)
	require.NotEmpty(t, menu.InlineKeyboard[0][0].URL)
	require.NotEmpty(t, menu.InlineKeyboard[0][1].URL)
	require.Len(t, menu.InlineKeyboard[1], 1)
	require.NotEmpty(t, menu.InlineKeyboard[1][0].URL)

	require.Len(t, menu.InlineKeyboard[2], 1)
	require.Equal(t, "users_back_platforms", menu.InlineKeyboard[2][0].Unique)
}

func TestKeyboard_Macos(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().Macos()

	require.Len(t, menu.InlineKeyboard, 3)
	require.Len(t, menu.InlineKeyboard[0], 2)
	require.NotEmpty(t, menu.InlineKeyboard[0][0].URL)
	require.NotEmpty(t, menu.InlineKeyboard[0][1].URL)
	require.Len(t, menu.InlineKeyboard[1], 1)
	require.NotEmpty(t, menu.InlineKeyboard[1][0].URL)

	require.Len(t, menu.InlineKeyboard[2], 1)
	require.Equal(t, "users_back_platforms", menu.InlineKeyboard[2][0].Unique)
}

func TestKeyboard_Back(t *testing.T) {
	t.Parallel()

	menu := newTestKeyboard().Back()

	require.Len(t, menu.InlineKeyboard, 1)
	require.Len(t, menu.InlineKeyboard[0], 1)
	require.Equal(t, "users_back", menu.InlineKeyboard[0][0].Unique)
}
