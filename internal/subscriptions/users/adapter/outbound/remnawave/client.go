package remnawave

import (
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	"go.uber.org/zap"
)

type RemnawaveAdapter struct {
	apiKey        string
	squadUUID     string
	client        *platformremnawave.Client
	notifierAdmin *ErrorHandler
}

func NewRemnawaveClient(
	client *platformremnawave.Client,
	apiKey string,
	squadUUID string,
	logger *zap.Logger,
	notifier Notifier,
) *RemnawaveAdapter {
	return &RemnawaveAdapter{
		client:        client,
		apiKey:        apiKey,
		squadUUID:     squadUUID,
		notifierAdmin: NewErrorHandler(logger, notifier),
	}
}
