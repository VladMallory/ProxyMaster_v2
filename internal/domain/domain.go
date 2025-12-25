package domain

import (
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
