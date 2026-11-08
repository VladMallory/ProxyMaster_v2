package telegramhandler

import (
	"testing"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

func TestTermLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		printPeriod string
		term        subdomain.Term
		want        string
	}{
		{
			name:        "месяцы: 30 дней превращаются в один месяц",
			printPeriod: "months",
			term:        subdomain.Days(30),
			want:        "1 мес.",
		},
		{
			name:        "месяцы: 60 дней превращаются в два месяца",
			printPeriod: "months",
			term:        subdomain.Days(60),
			want:        "2 мес.",
		},
		{
			name:        "месяцы: 120 дней превращаются в четыре месяца",
			printPeriod: "months",
			term:        subdomain.Days(120),
			want:        "4 мес.",
		},
		{
			name:        "месяцы: 45 дней остаются днями",
			printPeriod: "months",
			term:        subdomain.Days(45),
			want:        "45 дн.",
		},
		{
			name:        "дни: 30 дней остаются днями",
			printPeriod: "days",
			term:        subdomain.Days(30),
			want:        "30 дн.",
		},
		{
			name:        "пустой режим: как дни",
			printPeriod: "",
			term:        subdomain.Days(30),
			want:        "30 дн.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &Handler{printPeriod: tt.printPeriod}
			if got := h.termLabel(tt.term); got != tt.want {
				t.Fatalf("termLabel(%+v) = %q, want %q", tt.term, got, tt.want)
			}
		})
	}
}
