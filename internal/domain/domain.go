package domain

import "context"

// RemnawaveClient - то как мы хотим получать информацию
type RemnawaveClient interface {
	GetServiceInfo(ctx context.Context, serviceID string) (string, error)
}
