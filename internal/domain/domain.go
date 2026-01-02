package domain

import (
	"context"
	"errors"

	"ProxyMaster_v2/internal/models"
)

var (
	// ErrInsufficientFunds ошибка недостаточного баланса
	ErrInsufficientFunds = errors.New("insufficient funds")
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
	CreateUser(models.CreateUserTGDTO) (*models.UserTG, error)
	GetAllUsers() ([]models.UserTG, error)
	GetUserByID(string) (*models.UserTG, error)
	UpdateUser(string, models.UpdateUserTGDTO) (*models.UserTG, error)
}

// SubscriptionService - бизнес логика управления подписками
type SubscriptionService interface {
	// ActivateSubscriotion обрабатывает логику создания или
	// продления подписки
	// принимает телеграм id и на сколько месяцев нужно
	ActivateSubscription(telegramID int64, months int) (string, error)
}

// TrialService - бизнес логика пробного периода
type TrialService interface {
	ActivateTrial(telegramID int64) (string, error)
}
