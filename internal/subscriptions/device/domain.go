package device

import (
	"errors"
	"time"
)

// Виды мест. Базовые входят в подписку и продлеваются вместе с ней,
// дополнительные покупаются отдельно и живут своим сроком.
const (
	KindBase  = "base"
	KindExtra = "extra"
)

var (
	ErrSeatNotFree        = errors.New("место уже занято или недоступно")
	ErrSeatNotFound       = errors.New("место не найдено")
	ErrPanelNotFound      = errors.New("пользователь или устройство не найдены в панели")
	ErrSeatNotRenewable   = errors.New("место нельзя продлить")
	ErrDeviceLimitReached = errors.New("достигнут лимит устройств")
	ErrSubscriptionClosed = errors.New("подписка не активна")
	errEmptyUsername      = errors.New("пустое поле username")
)

type Device struct {
	ID int64
	Username,
	Label,
	HWID,
	Kind string
	ExpiresAt,
	RevokedAt,
	CreatedAt time.Time
}

func (d Device) Bound() bool { return d.HWID != "" }

func (d Device) Revoked() bool { return !d.RevokedAt.IsZero() }

func (d Device) Base() bool { return d.Kind == KindBase }

func (d Device) Extra() bool { return d.Kind == KindExtra }

func (d Device) ActiveAt(t time.Time) bool {
	return !d.Revoked() && d.ExpiresAt.After(t)
}

func (d Device) ExpiredAt(t time.Time) bool {
	return !d.Revoked() && !d.ExpiresAt.After(t)
}

func (d Device) FreeAt(t time.Time) bool {
	return d.ActiveAt(t) && !d.Bound()
}

func NewExpire(now time.Time, months int) time.Time {
	return now.AddDate(0, months, 0)
}

// ExtendFrom прибавляет месяцы к текущему сроку, но не раньше now:
// просроченный остаток не сгорает.
func ExtendFrom(current, now time.Time, months int) time.Time {
	base := current
	if base.Before(now) {
		base = now
	}

	return base.AddDate(0, months, 0)
}

func CountActive(devices []Device, now time.Time) int {
	var n int

	for _, d := range devices {
		if d.ActiveAt(now) {
			n++
		}
	}

	return n
}

// CountActiveExtra считает активные оплаченные места: они влияют на лимит
// панели сверх базового числа.
func CountActiveExtra(devices []Device, now time.Time) int {
	var n int

	for _, d := range devices {
		if d.Extra() && d.ActiveAt(now) {
			n++
		}
	}

	return n
}

func FirstFreeIndex(devices []Device, now time.Time) int {
	for i := range devices {
		if devices[i].FreeAt(now) {
			return i
		}
	}

	return -1
}
