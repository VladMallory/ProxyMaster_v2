package remnawave

import (
	"context"
	"errors"
	"fmt"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"go.uber.org/zap"
)

type ErrorMeta struct {
	Op       string // имя операции, например "GetByUsername"
	Username string
}

type Notifier interface {
	Notify(ctx context.Context, err error, meta ErrorMeta)
}

type ErrorHandler struct {
	logger   *zap.Logger
	notifier Notifier
}

func NewErrorHandler(logger *zap.Logger, notifier Notifier) *ErrorHandler {
	return &ErrorHandler{logger: logger, notifier: notifier}
}

func (h *ErrorHandler) Map(ctx context.Context, err error, meta ErrorMeta) error {
	if err == nil {
		return nil
	}

	h.logger.Error(
		"remnawave unexpected",
		zap.String("op", meta.Op),
		zap.String("username", meta.Username),
		zap.Error(err),
	)

	if h.notifier != nil {
		h.notifier.Notify(ctx, err, meta)
	}

	if errors.Is(err, platformremnawave.ErrNotFound) {
		return subdomain.ErrNoFindUser
	}

	return fmt.Errorf("%s: %w", meta.Op, err)
}
