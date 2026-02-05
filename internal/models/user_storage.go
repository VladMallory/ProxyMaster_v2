// Package models содержит структуры данных для работы с БД и внешними DTO.
package models

import "time"

// UserTG описывает пользователя в нашей БД (телеграм-пользователь).
type UserTG struct {
	// ID телеграм-id пользователя (наш username).
	ID string `db:"id"`

	// Trial флаг пробного периода.
	Trial bool `db:"trial"`

	// ExtraDevicesCount удобный счетчик активных купленных доп. устройств.
	ExtraDevicesCount int `db:"extra_devices_count"`

	// CreatedAt дата создания пользователя.
	CreatedAt time.Time `db:"created_at"`
}

// CreateUserTGDTO описывает данные для создания пользователя.
type CreateUserTGDTO struct {
	ID    string `db:"id"`
	Trial bool   `db:"trial"`
}

// UpdateUserTGDTO описывает частичное обновление пользователя.
type UpdateUserTGDTO struct {
	// Ставим * для надежности. Чтобы передавали значения через &
	// Иначе не указав явно Trial он бы при каждом запросе
	// ставился бы на false. А так он остается таким же, как был.
	// Потому что ставится nil, если значение не указано.
	Trial *bool

	// ExtraDevicesCount количество активных доп. устройств.
	ExtraDevicesCount *int
}
