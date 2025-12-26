package domain

import (
	"ProxyMaster_v2/internal/models"
	"context"
)

// RemnawaveClient - то как мы хотим получать информацию
type RemnawaveClient interface {
	Login(ctx context.Context, username string, password string) error
	GetServiceInfo(ctx context.Context, serviceID string) (string, error)
	GetUUIDByUsername(username string) (string, error)
	CreateUser(username string, days int) error
	ExtendClientSubscription(userUUID string, days int) error
	EnableClient(userUUID string) error
	DisableClient(userUUID string) error
}

type UserRepository interface {
	CreateUser(models.CreateUserTGDTO) (models.UserTG, error)
	GetAllUsers() []models.UserTG
	GetUserByID(string) (*models.UserTG, error)
	//UpdateUser
	UpdateUser(string, models.UpdateUserTGDTO) (*models.UserTG, error)
}
