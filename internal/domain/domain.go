package domain

import (
	"ProxyMaster_v2/internal/models"
	"context"
	"errors"
)

var (
	// ErrInsufficientFunds ошибка недостаточного баланса.
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrUserNotFound         = errors.New("user not found")
	ErrMaxDevices           = errors.New("max devices")
	ErrNoActiveDeviceAddons = errors.New("no active device addons")
)

// ViewType тип ошибки
type ViewType string

// ViewType какие ошибки могут быть
const (
	ViewTypeDownloadApp        ViewType = "download_app"
	ViewTypeIosRegion          ViewType = "ios_region"
	ViewTypeTariffs            ViewType = "tariffs"
	ViewTypeTopUp              ViewType = "top_up"
	ViewTypePayment            ViewType = "payment"
	ViewTypeCheckPayment       ViewType = "check_payment"
	ViewTypeProfile            ViewType = "profile"
	ViewTypeDeviceLimits       ViewType = "device_limits"
	ViewTypeTrafficLimits      ViewType = "traffic_limits"
	ViewTypeConnect            ViewType = "connect"
	ViewTypeSubscriptionResult ViewType = "subscription_result"
	ViewTypeMain               ViewType = "main"
	ViewTypeServiceInfo        ViewType = "service_info"
	ViewTypePrivacyPolicy      ViewType = "privacy_policy"
	ViewTypeUserAgreement      ViewType = "user_agreement"
	ViewTypeError              ViewType = "unknown view type"
)

// Ошибка приложения.
var (
	ErrDuplicateKey       = errors.New("duplicate key")
	ErrDatabaseConnection = errors.New("database connection error")
)

// RemnawaveClient то как мы хотим получать информацию.
type RemnawaveClient interface {
	GetUUIDByUsername(ctx context.Context, username string) (string, error)
	CreateUser(username string, days int) error
	DeleteUser(ctx context.Context, username string) error
	ExtendClientSubscription(userUUID string, username string, days int) error
	EnableClient(userUUID string) error
	DisableClient(userUUID string) error
	GetUserInfo(uuid string) (models.GetUserInfoResponse, error)
	SetDevices(ctx context.Context, username string, devices *uint8) error
	SetTraffic(username string, gb uint64) error
	AddTraffic(username string, gb uint64) error
	BetterResetTraffic(ctx context.Context, username string) error
}

type UserRepository interface {
	CreateUser(models.CreateUserTGDTO) (*models.UserTG, error)
	GetAllUsers() ([]models.UserTG, error)
	GetActiveUserIDs() ([]string, error)
	GetUserByID(id string) (*models.UserTG, error)
	UpdateUser(id string, updateData models.UpdateUserTGDTO) (*models.UserTG, error)
}

// SubscriptionService - бизнес логика управления подписками
type SubscriptionService interface {
	// ActivateSubscription обрабатывает логику создания или
	// продления подписки
	// принимает телеграм id и на сколько месяцев нужно
	ActivateSubscription(telegramID int64, months int) (string, error)

	// AddDevice добавляет 1 устройство пользователю
	AddDevice(username string) error

	// AddPaidDevice покупает 1 дополнительные устройство за 50₽/мес
	AddPaidDevice(username string) error

	// ResetPaidDevices сбрасывает дополнительные устройства до 0
	ResetPaidDevices(username string) error

	PrepayPaidDevices(username string) (int, error)

	GetURLSubscription(username string) (string, error)
}

// TrialService бизнес логика пробного периода.
type TrialService interface {
	ActivateTrial(telegramID int64) (string, error)
}

// ServerAPI интерфейс для работы с сервером.
type ServerAPI interface {
	Serve(string) error
}
