package telegram

// SubscriptionService — что transport-слой хочет от сервиса подписок.
type SubscriptionService interface {
	ActivateSubscription(userID string, months int) (string, error)
	GetURLSubscription(userID string) (string, error)
}

// DeviceService — что transport-слой хочет от сервиса устройств.
type DeviceService interface {
	AddPaidDevice(userID string) error
	ResetPaidDevices(userID string) error
	PrepayPaidDevices(userID string) (int, error)
}
