package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	platformHTTP "github.com/VladMallory/ProxyMaster_v2/internal/platform/http"
	"go.uber.org/zap"
)

// SubscriptionService — интерфейс, который нужен HTTP хендлерам.
// Определяем его здесь же, чтобы не зависеть от конкретной реализации.
type SubscriptionService interface {
	GetURLSubscription(userID string) (string, error)
	ActivateSubscription(userID string, months int) (string, error)
}

// Handler — HTTP хендлеры для подписок.
type Handler struct {
	subSvc SubscriptionService
	logger *zap.Logger
}

func NewHandler(subSvc SubscriptionService, logger *zap.Logger) *Handler {
	return &Handler{
		subSvc: subSvc,
		logger: logger,
	}
}

// request — общие структуры запросов.
type activateRequest struct {
	UserID string `json:"user_id"`
	Months int    `json:"months"`
}

// GetSubscriptionURL — GET /api/v1/subscription/{userID}/url.
func (h *Handler) GetSubscriptionURL(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	if userID == "" {
		platformHTTP.RespondError(w, http.StatusBadRequest, "user_id is required")

		return
	}

	url, err := h.subSvc.GetURLSubscription(userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			platformHTTP.RespondError(w, http.StatusNotFound, "user not found")

			return
		}
		platformHTTP.RespondError(
			w, http.StatusInternalServerError,
			"failed to get subscription URL",
		)

		return
	}

	platformHTTP.RespondJSON(w, http.StatusOK, map[string]string{
		"subscription_url": url,
	})
}

// ActivateSubscription — POST /api/v1/subscription/activate.
func (h *Handler) ActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req activateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		platformHTTP.RespondError(w, http.StatusBadRequest, "invalid request body")

		return
	}

	if req.UserID == "" {
		platformHTTP.RespondError(w, http.StatusBadRequest, "user_id is required")

		return
	}
	if req.Months <= 0 {
		platformHTTP.RespondError(w, http.StatusBadRequest, "months must be positive")

		return
	}

	result, err := h.subSvc.ActivateSubscription(req.UserID, req.Months)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			platformHTTP.RespondError(w, http.StatusNotFound, "user not found")

			return
		}
		platformHTTP.RespondError(
			w,
			http.StatusInternalServerError,
			"failed to activate subscription",
		)

		return
	}

	platformHTTP.RespondJSON(w, http.StatusOK, map[string]string{
		"message": result,
	})
}
