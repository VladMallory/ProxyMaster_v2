package domain

import (
	"errors"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

// UserTG описывает пользователя в нашей БД (телеграм-пользователь).
type UserTG struct {
	ID                string    `db:"id"`
	Trial             bool      `db:"trial"`
	ExtraDevicesCount int       `db:"extra_devices_count"`
	CreatedAt         time.Time `db:"created_at"`
}

// CreateUserDTO описывает данные для создания пользователя.
type CreateUserDTO struct {
	ID    string `db:"id"`
	Trial bool   `db:"trial"`
}

// UpdateUserTGDTO описывает частичное обновление пользователя.
type UpdateUserTGDTO struct {
	Trial             *bool
	ExtraDevicesCount *int
}

// UserRepository определяет операции работы с пользователями в БД.
type UserRepository interface {
	CreateUser(userData CreateUserDTO) (*UserTG, error)
	GetUserByID(id string) (*UserTG, error)
	UpdateUser(id string, updateData UpdateUserTGDTO) (*UserTG, error)
}
