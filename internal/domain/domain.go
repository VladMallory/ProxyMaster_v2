package domain

import (
	"context"

	"ProxyMaster_v2/internal/models"
)

// RemnawaveClient - то как мы хотим получать информацию
type RemnawaveClient interface {
	Login(ctx context.Context, username string, password string) error
	GetUUIDByUsername(username string) (string, error)
	CreateUser(username string, days int) error
	ExtendClientSubscription(userUUID string, username string, days int) error
	EnableClient(userUUID string) error
	DisableClient(userUUID string) error
	GetUserInfo(uuid string) (models.GetUserInfoResponse, error)
}

type UserRepository interface {
	CreateUser(models.CreateUserTGDTO) (models.UserTG, error)
	GetAllUsers() []models.UserTG
	GetUserByID(string) (*models.UserTG, error)
	// UpdateUser
	UpdateUser(string, models.UpdateUserTGDTO) (*models.UserTG, error)
}

// SubscriptionService - бизнес логика управления подписками
type SubscriptionService interface {
	// ActivateSubscriotion обрабатывает логику создания или
	// продления подписки
	// принимает телеграм id и на сколько месяцев нужно
	ActivateSubscription(telegramID int64, months int) (string, error)
}
