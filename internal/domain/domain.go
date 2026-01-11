package domain

import (
	"context"
	"errors"

	"ProxyMaster_v2/internal/models"
)

var (
	// ErrInsufficientFunds ошибка недостаточного баланса
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrUserNotFound      = errors.New("user not found")
	ErrMaxDevices        = errors.New("max devices")
)

type ViewType string

const (
	ViewTypeDownloadApp        ViewType = "download_app"
	ViewTypeIosRegion          ViewType = "ios_region"
	ViewTypeTariffs            ViewType = "tariffs"
	ViewTypeTopup              ViewType = "topup"
	ViewTypePayment            ViewType = "payment"
	ViewTypeCheckPayment       ViewType = "check_payment"
	ViewTypeProfile            ViewType = "profile"
	ViewTypeDeviceLimits       ViewType = "device_limits"
	ViewTypeTrafficLimits      ViewType = "traffic_limits"
	ViewTypeConnect            ViewType = "connect"
	ViewTypeSubscriptionResult ViewType = "subscription_result"
	ViewTypeMain               ViewType = "main"
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
	SetDevices(username string, devices *uint8) error
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

	// AddDevice добавляет 1 устройство пользователю
	AddDevice(username string) error

	// AddPaidDevice покупает 1 доп. устройство за 50₽/мес
	AddPaidDevice(username string) error

	// ResetPaidDevices сбрасывает доп. устройства до 0
	ResetPaidDevices(username string) error
}

// TrialService - бизнес логика пробного периода
type TrialService interface {
	ActivateTrial(telegramID int64) (string, error)
}

// ServerAPI - интерфейс для работы с сервером
type ServerAPI interface {
	Serve(string) error
}
