package domain

import "time"

// DeviceAddon описывает одну купленную пользователем "доп. услугу" — дополнительное устройство.
type DeviceAddon struct {
	// ID уникальный идентификатор записи (uuid строкой).
	ID string `db:"id"`

	// UserID телеграм-id пользователя (наш username).
	UserID string `db:"user_id"`

	// NextChargeAt дата следующего списания 50₽ за эту конкретную покупку.
	NextChargeAt time.Time `db:"next_charge_at"`

	// Active флаг активности услуги (false = отключено/сброшено).
	Active bool `db:"active"`

	// CreatedAt дата создания записи.
	CreatedAt time.Time `db:"created_at"`
}
