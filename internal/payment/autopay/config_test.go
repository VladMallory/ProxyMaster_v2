package autopay

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "в верхнем регистре", value: "TRUE", want: true},
		{name: "false", value: "false", want: false},
		{name: "пусто", value: "", want: false},
		{name: "другое значение", value: "yes", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("YOOKASSA_AUTOPAY", tt.value)
			require.Equal(t, tt.want, Enabled())
		})
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	t.Setenv("YOOKASSA_AUTOPAY", "true")
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("YOUKASSA_SHOP_ID", "shop")
	t.Setenv("YOUKASSA_SECRET_KEY", "secret")
	t.Setenv("YOUKASSA_RETURN_URL", "https://return.example")
	t.Setenv("REMNA_PANEL", "https://panel.example?key=abc")
	t.Setenv("REMNA_TOKEN", "token")
	t.Setenv("TELEGRAM_TOKEN", "tg-token")
	t.Setenv("PRICE_PER_MONTH", "100")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, Config{
		Autopay:       true,
		DatabaseURL:   "postgres://db",
		ShopID:        "shop",
		SecretKey:     "secret",
		ReturnURL:     "https://return.example",
		PanelBaseURL:  "https://panel.example",
		PanelAPIKey:   "key=abc",
		PanelToken:    "token",
		TelegramToken: "tg-token",
		PricePerMonth: 100,
	}, cfg)
}

//nolint:paralleltest // t.Setenv запрещает параллельные тесты.
func TestLoadConfig_RejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T)
	}{
		{
			name: "пустой DATABASE_URL",
			prepare: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "")
			},
		},
		{
			name: "пустые ключи ЮKassa",
			prepare: func(t *testing.T) {
				t.Setenv("YOUKASSA_SHOP_ID", "")
			},
		},
		{
			name: "пустой REMNA_TOKEN",
			prepare: func(t *testing.T) {
				t.Setenv("REMNA_TOKEN", "")
			},
		},
		{
			name: "цена не число",
			prepare: func(t *testing.T) {
				t.Setenv("PRICE_PER_MONTH", "abc")
			},
		},
		{
			name: "цена не положительная",
			prepare: func(t *testing.T) {
				t.Setenv("PRICE_PER_MONTH", "0")
			},
		},
		{
			name: "панель без ключа",
			prepare: func(t *testing.T) {
				t.Setenv("REMNA_PANEL", "https://panel.example")
			},
		},
	}

	//nolint:paralleltest // t.Setenv запрещает параллельные тесты.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			tt.prepare(t)

			_, err := LoadConfig()
			require.Error(t, err)
		})
	}
}

func setValidEnv(t *testing.T) {
	t.Helper()

	t.Setenv("YOOKASSA_AUTOPAY", "true")
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("YOUKASSA_SHOP_ID", "shop")
	t.Setenv("YOUKASSA_SECRET_KEY", "secret")
	t.Setenv("YOUKASSA_RETURN_URL", "https://return.example")
	t.Setenv("REMNA_PANEL", "https://panel.example?key=abc")
	t.Setenv("REMNA_TOKEN", "token")
	t.Setenv("TELEGRAM_TOKEN", "tg-token")
	t.Setenv("PRICE_PER_MONTH", "100")
}

func TestParsePanel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantBase string
		wantKey  string
		wantErr  bool
	}{
		{
			name:     "валидный адрес",
			raw:      "https://panel.example?key=abc",
			wantBase: "https://panel.example",
			wantKey:  "key=abc",
		},
		{
			name:    "без схемы",
			raw:     "panel.example?key=abc",
			wantErr: true,
		},
		{
			name:    "без ключа",
			raw:     "https://panel.example",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base, key, err := parsePanel(tt.raw)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantBase, base)
			require.Equal(t, tt.wantKey, key)
		})
	}
}
