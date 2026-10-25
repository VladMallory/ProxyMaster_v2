//nolint:cyclop,funlen,golines,intrange,lll,nlreturn,paralleltest,staticcheck
package device

import (
	"testing"
	"time"
)

func TestSeatStates(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	table := []struct {
		name   string
		seat   Device
		active bool
		free   bool
		expire bool
		bound  bool
	}{
		{
			name:   "оплаченное свободное место",
			seat:   Device{ExpiresAt: now.Add(time.Hour)},
			active: true,
			free:   true,
		},
		{
			name:   "оплаченное занятое место",
			seat:   Device{ExpiresAt: now.Add(time.Hour), HWID: "a"},
			active: true,
			bound:  true,
		},
		{
			name:   "срок истекает ровно сейчас",
			seat:   Device{ExpiresAt: now},
			expire: true,
		},
		{
			name:   "просроченное место не свободно",
			seat:   Device{ExpiresAt: now.Add(-time.Second)},
			expire: true,
		},
		{
			name: "отозванное место не активно и не ждёт отзыва",
			seat: Device{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: now.Add(-time.Minute),
			},
		},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.seat.ActiveAt(now); got != tc.active {
				t.Fatalf("ActiveAt = %v, want %v", got, tc.active)
			}
			if got := tc.seat.FreeAt(now); got != tc.free {
				t.Fatalf("FreeAt = %v, want %v", got, tc.free)
			}
			if got := tc.seat.ExpiredAt(now); got != tc.expire {
				t.Fatalf("ExpiredAt = %v, want %v", got, tc.expire)
			}
			if got := tc.seat.Bound(); got != tc.bound {
				t.Fatalf("Bound = %v, want %v", got, tc.bound)
			}
		})
	}
}

func TestCountActiveCountsFreeSeats(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	seats := []Device{
		{ExpiresAt: now.Add(time.Hour)},
		{ExpiresAt: now.Add(time.Hour), HWID: "a"},
		{ExpiresAt: now},
		{ExpiresAt: now.Add(-time.Hour), HWID: "b"},
		{ExpiresAt: now.Add(48 * time.Hour), RevokedAt: now},
	}

	if got := CountActive(seats, now); got != 2 {
		t.Fatalf("CountActive = %d, want 2", got)
	}
}

func TestCountActiveExtraCountsOnlyPaidSeats(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	seats := []Device{
		{ExpiresAt: now.Add(time.Hour), Kind: KindBase},
		{ExpiresAt: now.Add(time.Hour), Kind: KindExtra},
		{ExpiresAt: now.Add(2 * time.Hour), Kind: KindExtra, HWID: "a"},
		{ExpiresAt: now, Kind: KindExtra},
		{ExpiresAt: now.Add(time.Hour), Kind: KindExtra, RevokedAt: now},
	}

	if got := CountActiveExtra(seats, now); got != 2 {
		t.Fatalf("CountActiveExtra = %d, want 2", got)
	}
}

func TestFirstFreeIndexSkipsUnavailableSeats(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	seats := []Device{
		{ID: 1, ExpiresAt: now.Add(time.Hour), HWID: "a"},
		{ID: 2, ExpiresAt: now},
		{ID: 3, ExpiresAt: now.Add(time.Hour), RevokedAt: now},
		{ID: 4, ExpiresAt: now.Add(time.Hour)},
		{ID: 5, ExpiresAt: now.Add(time.Hour)},
	}

	if got := FirstFreeIndex(seats, now); got != 3 {
		t.Fatalf("FirstFreeIndex = %d, want 3", got)
	}
	if got := FirstFreeIndex(seats[:3], now); got != -1 {
		t.Fatalf("FirstFreeIndex = %d, want -1", got)
	}
}
