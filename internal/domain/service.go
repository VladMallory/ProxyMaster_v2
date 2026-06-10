package domain

// SubscriptionService - бизнес логика управления подписками.
type SubscriptionService interface {
	ActivateSubscription(userID string, months int) (string, error)
	GetURLSubscription(userID string) (string, error)
}

// DeviceService - бизнес логика управления устройствами.
type DeviceService interface {
	AddPaidDevice(userID string) error
	ResetPaidDevices(userID string) error
	PrepayPaidDevices(userID string) (int, error)
}

// TrialService бизнес логика пробного периода.
type TrialService interface {
	ActivateTrial(telegramID int64) (string, error)
}
