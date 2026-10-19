package device

import "time"

type Device struct {
	ID       int64
	Username string
	HWID     string
	// Label снимок «платформа / модель».
	Label string
	// ExpiresAt срок оплаты именно этого места.
	ExpiresAt time.Time
	// RevokedAt ненулевое, когда место истекло и устройство удалено из панели.
	RevokedAt time.Time
	// CreatedAt время покупки. Определяет порядок привязки (раньше куплено,
	// раньше получает подключившийся hwid).
	CreatedAt time.Time
}

// Revoked отозвано ли место (устройство уже удалено из панели).
func (d Device) Revoked() bool { return !d.RevokedAt.IsZero() }

// ActiveAt оплачено ли место на момент t (и ещё не отозвано).
func (d Device) ActiveAt(t time.Time) bool {
	return !d.Revoked() && d.ExpiresAt.After(t)
}

// Free свободно ли место: оплачено, ещё не отозвано и без привязанного hwid.
func (d Device) Free() bool { return !d.Revoked() && d.HWID == "" }

// NewExpiry срок нового места: ровно months календарных месяцев с оплаты.
func NewExpiry(now time.Time, months int) time.Time {
	return now.AddDate(0, months, 0)
}

// ExtendFrom новый срок при продлении: от большего из (текущий срок, now).
func ExtendFrom(current, now time.Time, months int) time.Time {
	base := current
	if base.Before(now) {
		base = now
	}

	return base.AddDate(0, months, 0)
}

// CountActive сколько мест оплачено на момент now. Это же число уходит в панель
// как hwidDeviceLimit: свободные, но оплаченные места тоже считаются.
func CountActive(ds []Device, now time.Time) int {
	n := 0
	for _, d := range ds {
		if d.ActiveAt(now) {
			n++
		}
	}

	return n
}

// MaxExpiresAt самый поздний срок среди активных мест. Нулевое, если активных
// нет тогда панели отправляем срок «в прошлом», чтобы подписка погасла.
func MaxExpiresAt(ds []Device, now time.Time) time.Time {
	var latest time.Time

	for _, d := range ds {
		if d.ActiveAt(now) && d.ExpiresAt.After(latest) {
			latest = d.ExpiresAt
		}
	}

	return latest
}

// FirstFreeIndex индекс самого раннего свободного места или -1.
//
// Ожидается, что ds отсортирован репозиторием по (created_at, id) тогда
// «самое раннее» это просто первое свободное. Возвращаем индекс, а не значение,
// чтобы вызывающий мог обновить копию в памяти без второго поиска.
func FirstFreeIndex(ds []Device) int {
	for i := range ds {
		if ds[i].Free() {
			return i
		}
	}

	return -1
}
